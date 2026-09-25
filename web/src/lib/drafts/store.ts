const databaseName = "quizzivy-authoring-drafts";
const lifetime = 7 * 24 * 60 * 60 * 1000;
let connection: Promise<IDBDatabase> | null = null;

export interface LocalDraft {
  owner: string;
  item: string;
  token: string;
  createdAt: number;
  expiresAt: number;
  payload: unknown;
}

function database(): Promise<IDBDatabase> {
  connection ??= new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open(databaseName, 1);
    request.onupgradeneeded = () => {
      request.result.createObjectStore("drafts", { keyPath: ["owner", "item"] });
      request.result.createObjectStore("metadata");
    };
    request.onsuccess = () => {
      request.result.onversionchange = () => {
        request.result.close();
        connection = null;
      };
      resolve(request.result);
    };
    request.onerror = () => {
      connection = null;
      reject(request.error);
    };
    request.onblocked = () => {
      connection = null;
      reject(new Error("Draft database blocked"));
    };
  });
  return connection;
}

function transact<T>(
  db: IDBDatabase,
  action: (transaction: IDBTransaction, result: (value: T) => void) => void,
): Promise<T> {
  return new Promise((resolve, reject) => {
    const transaction = db.transaction(["drafts", "metadata"], "readwrite");
    let value: T;
    transaction.oncomplete = () => resolve(value);
    transaction.onabort = () =>
      reject(transaction.error ?? new Error("Draft transaction aborted"));
    transaction.onerror = () => reject(transaction.error);
    action(transaction, (next) => {
      value = next;
    });
  });
}

/** openDraftScope isolates local authoring data by account and fences writes against logout and a newer editor of the same item in any tab. */
export async function openDraftScope(owner: string, item: string) {
  const db = await database();
  const lease = crypto.randomUUID();
  const leaseKey = JSON.stringify(["writer", owner, item]);
  const epoch = await transact<string>(db, (transaction, result) => {
    const metadata = transaction.objectStore("metadata");
    metadata.put(lease, leaseKey);
    const request = metadata.get("epoch");
    request.onsuccess = () => {
      const current =
        typeof request.result === "string" ? request.result : crypto.randomUUID();
      metadata.put(current, "epoch");
      result(current);
    };
    const cursor = transaction.objectStore("drafts").openCursor();
    cursor.onsuccess = () => {
      const entry = cursor.result;
      if (!entry) return;
      const record = entry.value as LocalDraft;
      if (
        !Number.isFinite(record.createdAt) ||
        !Number.isFinite(record.expiresAt) ||
        record.createdAt > Date.now() ||
        record.expiresAt > record.createdAt + lifetime ||
        record.expiresAt <= Date.now()
      )
        entry.delete();
      entry.continue();
    };
  });
  function operate<T>(
    action: (store: IDBObjectStore, result: (value: T) => void) => void,
  ) {
    return transact<T>(db, (transaction, result) => {
      authorizeWriter(transaction, epoch, leaseKey, lease, () => {
        try {
          action(transaction.objectStore("drafts"), result);
        } catch {
          transaction.abort();
        }
      });
    });
  }
  return {
    read: () =>
      operate<LocalDraft | null>((store, result) => {
        const request = store.get([owner, item]);
        request.onsuccess = () =>
          result((request.result as LocalDraft | undefined) ?? null);
      }),
    write: (token: string, createdAt: number, payload: unknown) =>
      operate<void>((store, result) => {
        if (
          new TextEncoder().encode(JSON.stringify(payload)).byteLength >
          4 * 1024 * 1024
        )
          throw new Error("Draft exceeds local recovery budget");
        const expiresAt = createdAt + lifetime;
        if (
          !Number.isFinite(createdAt) ||
          createdAt > Date.now() ||
          expiresAt <= Date.now()
        )
          throw new Error("Draft expired");
        store.put({
          owner,
          item,
          token,
          createdAt,
          expiresAt,
          payload,
        } satisfies LocalDraft);
        result();
      }),
    remove: () =>
      operate<void>((store, result) => {
        store.delete([owner, item]);
        result();
      }),
  };
}

export type DraftScope = Awaited<ReturnType<typeof openDraftScope>>;

/** clearAuthoringDrafts atomically clears unsent authoring data and invalidates pre-logout writers, including other tabs. */
export async function clearAuthoringDrafts(): Promise<void> {
  const db = await database();
  await transact<void>(db, (transaction, result) => {
    transaction.objectStore("metadata").clear();
    transaction.objectStore("metadata").put(crypto.randomUUID(), "epoch");
    transaction.objectStore("drafts").clear();
    result();
  });
}

function authorizeWriter(
  transaction: IDBTransaction,
  epoch: string,
  leaseKey: string,
  lease: string,
  action: () => void,
) {
  const metadata = transaction.objectStore("metadata");
  const session = metadata.get("epoch");
  const writer = metadata.get(leaseKey);
  writer.onsuccess = () => {
    if (session.result !== epoch || writer.result !== lease) {
      transaction.abort();
      return;
    }
    action();
  };
}
