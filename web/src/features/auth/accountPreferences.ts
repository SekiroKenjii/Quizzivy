import type { QueryClient } from "@tanstack/react-query";
import { beginAccountDeparture } from "./accountDeparture";
import { useSyncExternalStore } from "react";
import { toast } from "@/components/ui/sonner";
import i18n, { setLocale, type Locale } from "@/lib/i18n";
import { APP_TIME_ZONE, setDisplayTimeZone } from "@/lib/i18n/datetime";
import {
  setAccountTheme,
  writeThemePreference,
  type ThemePreference,
} from "@/lib/theme";
import { setAccountTestText, writeLargerTestText } from "@/lib/testText";
import { authStore, useAuthStore, type ActorLease } from "@/stores/auth";
import {
  fetchCurrentUser,
  updatePreferences,
  updateProfile,
  type PreferencesPatch,
  type ProfilePatch,
  type User,
} from "./api";
import {
  reserveTransition,
  finishTransition,
  isTransitionCurrent,
  waitForTransition,
  type AccountTransition,
} from "@/lib/api/authTransition";

/** PreferenceIntent is one existing account control's retained explicit choice. */
export type PreferenceIntent =
  { locale: Locale } | { theme: ThemePreference } | { largerTestText: boolean };

/** AccountPreferenceStatus distinguishes previews and failures from server acknowledgments. */
export interface AccountPreferenceStatus {
  phase: "idle" | "saving" | "saved" | "failed";
  intent: PreferenceIntent | null;
  unsupportedZone: string | null;
}

let status: AccountPreferenceStatus = {
  phase: "idle",
  intent: null,
  unsupportedZone: null,
};
const listeners = new Set<() => void>();
let lane: { generation: number; tail: Promise<unknown>; count: number } | null = null;
let compatibilityToast: string | number | undefined;
let choiceToast: string | number | undefined;

function publish(next: AccountPreferenceStatus) {
  status = next;
  for (const listener of listeners) listener();
}

function requireActor(lease: ActorLease) {
  if (!authStore.isCurrent(lease)) throw new DOMException("Superseded", "AbortError");
}

function applyUser(user: User) {
  void setLocale(user.locale ?? "vi");
  setAccountTheme(user.preferences?.theme ?? "light");
  setAccountTestText(user.preferences?.largerTestText ?? false);
  const supported = setDisplayTimeZone(user.timeZone ?? APP_TIME_ZONE);
  const unsupportedZone = supported ? null : (user.timeZone ?? null);
  if (status.unsupportedZone !== unsupportedZone)
    publish({ ...status, unsupportedZone });
  if (unsupportedZone) showCompatibility();
  else if (compatibilityToast !== undefined) {
    toast.dismiss(compatibilityToast);
    compatibilityToast = undefined;
  }
}

function showCompatibility() {
  if (!status.unsupportedZone || !useAuthStore.getState().user) return;
  const lease = authStore.captureActor();
  compatibilityToast = toast(i18n.t("settings.accountZoneUnsupported"), {
    id: `account-zone-${lease.generation}`,
    duration: Infinity,
    dismissible: false,
    action: {
      label: i18n.t("auth.transition.checkStatus"),
      onClick: () => {
        if (!authStore.isCurrent(lease)) return;
        void refreshAccount().catch(() => {
          if (authStore.isCurrent(lease)) showCompatibility();
        });
      },
    },
    onDismiss: () => {
      if (authStore.isCurrent(lease) && status.unsupportedZone)
        queueMicrotask(showCompatibility);
    },
  });
}

useAuthStore.subscribe((next, previous) => {
  if (next.actorGeneration !== previous.actorGeneration) {
    lane = null;
    if (choiceToast !== undefined) toast.dismiss(choiceToast);
    if (compatibilityToast !== undefined) toast.dismiss(compatibilityToast);
    choiceToast = undefined;
    compatibilityToast = undefined;
    publish({ phase: "idle", intent: null, unsupportedZone: null });
  }
  if (
    next.user &&
    (next.user !== previous.user || next.actorGeneration !== previous.actorGeneration)
  )
    applyUser(next.user);
  if (!next.user && previous.user) {
    setAccountTheme(null);
    setAccountTestText(null);
    setDisplayTimeZone(APP_TIME_ZONE);
  }
});
if (useAuthStore.getState().user) applyUser(useAuthStore.getState().user!);

async function ordered<T>(work: () => Promise<T>): Promise<T> {
  const lease = authStore.captureActor();
  if (!lease.userId) throw new DOMException("No account", "AbortError");
  if (!lane || lane.generation !== lease.generation)
    lane = { generation: lease.generation, tail: Promise.resolve(), count: 0 };
  const mine = lane;
  if (mine.count >= 2) throw new Error(i18n.t("settings.preferenceSaving"));
  mine.count += 1;
  const previous = mine.tail;
  const completion = previous
    .catch(() => undefined)
    .then(async () => {
      requireActor(lease);
      const result = await work();
      requireActor(lease);
      return result;
    });
  mine.tail = completion;
  try {
    return await completion;
  } finally {
    mine.count -= 1;
  }
}

function acceptUser(user: User, lease: ActorLease) {
  requireActor(lease);
  if (user.id !== lease.userId)
    throw new DOMException("Different account", "AbortError");
  useAuthStore.getState().setUser(user);
}

/** saveProfilePatch serializes actor-bound profile changes and accepts only the matching server user. */
export function saveProfilePatch(body: ProfilePatch) {
  const lease = authStore.captureActor();
  return ordered(async () => {
    const user = await updateProfile(
      Object.keys(body).length === 1 && typeof body.fullName === "string"
        ? body.fullName
        : body,
    );
    acceptUser(user, lease);
    return user;
  });
}

/** savePreferences serializes partial preference writes while preserving unrelated account properties. */
export function savePreferences(body: PreferencesPatch) {
  const lease = authStore.captureActor();
  return ordered(async () => {
    const preferences = await updatePreferences(body);
    requireActor(lease);
    const user = useAuthStore.getState().user;
    if (!user) throw new DOMException("No account", "AbortError");
    acceptUser({ ...user, preferences }, lease);
    return preferences;
  });
}

/** refreshAccount orders authoritative rereads with writes and rejects mismatched identities. */
export function refreshAccount() {
  const lease = authStore.captureActor();
  return ordered(async () => {
    const user = await fetchCurrentUser();
    acceptUser(user, lease);
    return user;
  });
}

/** runAccountMutation keeps a security mutation and its authoritative reread in one actor-bound lane. */
export function runAccountMutation(
  work: () => Promise<unknown>,
  bestEffortRead = false,
) {
  const lease = authStore.captureActor();
  return ordered(async () => {
    await work();
    requireActor(lease);
    try {
      const user = await fetchCurrentUser();
      acceptUser(user, lease);
      return user;
    } catch (error) {
      requireActor(lease);
      if (!bestEffortRead) {
        throw error;
      }
      return useAuthStore.getState().user;
    }
  });
}

function preview(intent: PreferenceIntent) {
  if ("locale" in intent) void setLocale(intent.locale);
  else if ("theme" in intent) writeThemePreference(intent.theme);
  else writeLargerTestText(intent.largerTestText);
}

/** chooseAccountPreference previews one changed key and retains an explicit retry on acknowledgment failure. */
export async function chooseAccountPreference(
  intent: PreferenceIntent,
): Promise<boolean> {
  const lease = authStore.captureActor();
  if (!lease.userId) {
    preview(intent);
    return true;
  }
  if (status.phase === "saving") return false;
  publish({ ...status, phase: "saving", intent });
  preview(intent);
  choiceToast = toast(i18n.t("settings.preferenceSaving"), {
    id: `account-choice-${lease.generation}`,
    duration: Infinity,
    action: null,
  });
  try {
    if ("locale" in intent) await saveProfilePatch(intent);
    else await savePreferences(intent);
    requireActor(lease);
    publish({ ...status, phase: "saved", intent: null });
    choiceToast = toast(i18n.t("settings.preferenceSaved"), {
      id: `account-choice-${lease.generation}`,
      duration: 4000,
      action: null,
    });
    return true;
  } catch {
    if (!authStore.isCurrent(lease)) return false;
    const user = useAuthStore.getState().user;
    if (user) applyUser(user);
    publish({ ...status, phase: "failed", intent });
    choiceToast = toast(i18n.t("settings.preferenceFailed"), {
      id: `account-choice-${lease.generation}`,
      duration: Infinity,
      action: {
        label: i18n.t("common.retry"),
        onClick: (event) => {
          if (authStore.isCurrent(lease)) {
            event.preventDefault();
            void chooseAccountPreference(intent);
          }
        },
      },
    });
    return false;
  }
}

/** useAccountPreferenceStatus exposes durable reopened-control status and compatibility recovery. */
export function useAccountPreferenceStatus() {
  return useSyncExternalStore(
    (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    () => status,
    () => status,
  );
}

/** retryAccountPreference resends only the current actor's retained explicit preference intention. */
export function retryAccountPreference() {
  return status.intent
    ? chooseAccountPreference(status.intent)
    : Promise.resolve(false);
}

/** signInAccount reserves transport before replacing an actor and discards timed-out late admission. */
export async function signInAccount<T extends { accessToken: string; user: User }>(
  kind: "login" | "google",
  work: (ticket: AccountTransition) => Promise<T>,
  queryClient: QueryClient,
  interested: () => boolean = () => true,
): Promise<T & { actor: ActorLease }> {
  const ticket = reserveTransition(kind, authStore.getGeneration());
  const departure = useAuthStore.getState().user
    ? beginAccountDeparture(ticket, queryClient, false)
    : null;
  if (!departure) useAuthStore.getState().clearSession();
  const lease = authStore.captureActor();
  if (departure) await waitForTransition(ticket, departure.clean());
  if (!isTransitionCurrent(ticket) || !authStore.isCurrent(lease) || !interested()) {
    finishTransition(ticket);
    throw new DOMException("Superseded", "AbortError");
  }
  const completion = work(ticket)
    .then((result) => {
      requireActor(lease);
      if (!isTransitionCurrent(ticket) || !interested())
        throw new DOMException("Superseded", "AbortError");
      useAuthStore.getState().setSession(result.accessToken, result.user);
      return { ...result, actor: authStore.captureActor() };
    })
    .finally(() => finishTransition(ticket));
  return waitForTransition(ticket, completion);
}
