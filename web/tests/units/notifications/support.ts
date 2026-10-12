import { createElement, type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";
import { HttpResponse } from "msw";
import { contractJson } from "@tests/support/contractResponse";
import type {
  Notification,
  NotificationPreference,
  NotificationPage,
} from "@/features/notifications/api";

export const BASE = "http://localhost:8080";
export function id(n: number) {
  return `00000000-0000-4000-8000-${n.toString(16).padStart(12, "0")}`;
}
export const assignmentId = id(1001);
export const attemptId = id(1002);
export const title = "Mid-term Reading Mock";
export const closesAt = "2026-10-01T23:30:00Z";
export const notices: Notification[] = [
  {
    kind: "attempt.submitted",
    params: { title, count: 8, toGrade: 6 },
    target: { route: "assignment", assignmentId },
  },
  {
    kind: "attempt.flagged",
    params: { title, studentName: "Lê Hoàng Nam", focusLost: 3 },
    target: { route: "attempt", attemptId },
  },
  {
    kind: "assignment.closing",
    params: { title, notSubmitted: 4 },
    target: { route: "assignment", assignmentId },
  },
  {
    kind: "class.joined",
    params: { studentName: "Nguyễn Gia Bảo", className: "IELTS Foundation A" },
    target: { route: "classes" },
  },
  {
    kind: "join_codes.rotated",
    params: {
      count: 7,
      classNames: [
        "IELTS Foundation A",
        "IELTS 6.5 Evening",
        "IELTS 7.0 Morning",
        "Reading A",
        "Listening B",
      ],
    },
    target: { route: "classes" },
  },
  {
    kind: "assignment.opened",
    params: { title, closesAt },
    target: { route: "studentAssignment", assignmentId },
  },
  {
    kind: "assignment.due_soon",
    params: { title: "Reading test", closesAt },
    target: { route: "studentAssignment", assignmentId },
  },
  {
    kind: "assignment.extended",
    params: { title, closesAt },
    target: { route: "studentAssignment", assignmentId },
  },
  {
    kind: "result.ready",
    params: { title: "Unit 3 grammar check" },
    target: { route: "result", attemptId },
  },
].map((row, index) => ({
  ...row,
  id: id(index + 1),
  createdAt: "2026-10-01T12:00:00Z",
  readAt: null,
})) as Notification[];
export const preferences: NotificationPreference[] = [
  "attempt.submitted",
  "attempt.flagged",
  "assignment.closing",
  "assignment.due_soon",
  "result.ready",
].map((event) => ({
  event: event as NotificationPreference["event"],
  inApp: true,
  email: false,
}));
export const page: NotificationPage = {
  items: Array.from({ length: 20 }, (_, index) => ({
    ...notices[index % notices.length]!,
    id: id(index + 1),
  })),
  nextBefore: id(20),
};
export function listResponse(value: NotificationPage = page) {
  return contractJson("/me/notifications", "get", 200, value);
}
export function summaryResponse(unreadNotifications = 3) {
  return contractJson("/me/summary", "get", 200, { unreadNotifications });
}
export function preferenceResponse(method: "get" | "put", rows = preferences) {
  return contractJson("/me/notification-preferences", method, 200, rows);
}
export function errorResponse(status = 500) {
  return HttpResponse.json(
    {
      error: {
        code: "REQUEST_INCOMPLETE",
        message: "Yêu cầu chưa đầy đủ.",
        requestId: id(999),
      },
    },
    { status },
  );
}
export function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((accept) => {
    resolve = accept;
  });
  return { promise, resolve };
}
const clients: QueryClient[] = [];
export function harness() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: 30_000, refetchOnWindowFocus: false },
      mutations: { retry: false },
    },
  });
  clients.push(client);
  const wrapper = ({ children }: Readonly<{ children: ReactNode }>) =>
    createElement(QueryClientProvider, { client }, children);
  return { client, wrapper };
}
afterEach(() => {
  cleanup();
  for (const client of clients.splice(0)) client.clear();
});
