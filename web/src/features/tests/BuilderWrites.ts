/** BuilderWrites serializes operations on one enclosing test; a rejected operation does not poison later explicit retries. */
export class BuilderWrites {
  private tail: Promise<unknown> = Promise.resolve();

  run<T>(operation: () => Promise<T>): Promise<T> {
    const result = this.tail.then(operation);
    this.tail = result.catch(() => undefined);
    return result;
  }
}
