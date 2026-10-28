// A client of the package compiles against it under each module resolution a
// client may use, with skipLibCheck off, and the types are not `any`
// (QC-SDK-1: extensionless specifiers in index.d.ts became `any` on node16).
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { test } from 'node:test';

const TSC = resolve('web/node_modules/typescript/bin/tsc');
const SDK = resolve('sdk');

const SOURCE = `import type { components, operations, paths, OperationId } from "@quizzivy/sdk";
import { CONTRACT_VERSION, OPERATIONS } from "@quizzivy/sdk";

type IsAny<T> = 0 extends 1 & T ? true : false;
export const notAny: [IsAny<components>, IsAny<paths>, IsAny<operations>, IsAny<OperationId>] = [false, false, false, false];
export const version: string = CONTRACT_VERSION;
export const login: { method: "post"; path: "/auth/login" } = OPERATIONS.login;
`;

for (const [module, moduleResolution] of [
  ['node16', 'node16'],
  ['nodenext', 'nodenext'],
  ['esnext', 'bundler'],
]) {
  test(`a ${moduleResolution} client sees the SDK's types`, () => {
    const dir = mkdtempSync(join(tmpdir(), 'sdk-consumer-'));
    try {
      mkdirSync(join(dir, 'node_modules', '@quizzivy'), { recursive: true });
      symlinkSync(SDK, join(dir, 'node_modules', '@quizzivy', 'sdk'), 'dir');
      writeFileSync(join(dir, 'package.json'), JSON.stringify({ type: 'module' }));
      writeFileSync(join(dir, 'index.ts'), SOURCE);
      writeFileSync(
        join(dir, 'tsconfig.json'),
        JSON.stringify({
          compilerOptions: { module, moduleResolution, target: 'es2022', strict: true, noEmit: true, skipLibCheck: false },
          files: ['index.ts'],
        }),
      );
      const result = spawnSync(process.execPath, [TSC, '-p', dir], { encoding: 'utf8' });
      assert.equal(result.status, 0, result.stdout + result.stderr);
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });
}
