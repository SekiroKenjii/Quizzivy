import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

import { bumpsBreaking, parseVersion, versionOf, violationsOf } from '../check.mjs';
import { operationsOf, renderModule, versionOf as documentVersion, withVersion } from '../gen.mjs';

const doc = (paths, version = '1.2.3') => ({ info: { version }, paths });

test('operationsOf lists operations in document order with method and path', () => {
  const ops = operationsOf(
    doc({
      '/a': { get: { operationId: 'getA' }, post: { operationId: 'makeA' }, parameters: [] },
      '/b/{id}': { delete: { operationId: 'dropB' } },
    }),
  );
  assert.deepEqual(ops, [
    { id: 'getA', method: 'get', path: '/a' },
    { id: 'makeA', method: 'post', path: '/a' },
    { id: 'dropB', method: 'delete', path: '/b/{id}' },
  ]);
});

test('operationsOf refuses a missing or repeated operationId', () => {
  assert.throws(() => operationsOf(doc({ '/a': { get: {} } })), /GET \/a has no operationId/);
  assert.throws(
    () => operationsOf(doc({ '/a': { get: { operationId: 'x' } }, '/b': { put: { operationId: 'x' } } })),
    /operationId x is used twice/,
  );
});

test('the generated version must be plain semver', () => {
  assert.equal(documentVersion(doc({}, '0.13.2')), '0.13.2');
  for (const bad of ['0.13', 'v0.13.2', '0.13.2-dev.1', undefined]) {
    assert.throws(() => documentVersion({ info: { version: bad } }), /info.version must be/);
  }
});

test('renderModule writes the same operations to the JS and the types', () => {
  const { js, dts } = renderModule(doc({ '/a': { get: { operationId: 'getA' } } }, '0.13.2'));
  assert.match(js, /export const CONTRACT_VERSION = "0\.13\.2";/);
  assert.match(js, /"getA": Object\.freeze\(\{ method: "get", path: "\/a" \}\)/);
  assert.match(dts, /export declare const CONTRACT_VERSION: "0\.13\.2";/);
  assert.match(dts, /readonly "getA": \{ readonly method: "get"; readonly path: "\/a" \};/);
  assert.match(dts, /export type OperationId = keyof typeof OPERATIONS;/);
});

test('withVersion changes only the version', () => {
  const before = '{\n  "name": "@quizzivy/sdk",\n  "version": "0.0.0",\n  "files": ["a"]\n}\n';
  assert.equal(
    withVersion(before, '0.13.2'),
    '{\n  "name": "@quizzivy/sdk",\n  "version": "0.13.2",\n  "files": [\n    "a"\n  ]\n}\n',
  );
});

test('versionOf reads info.version from the YAML, not a nested one', () => {
  const yaml = 'openapi: 3.1.0\n\ninfo:\n  title: X\n  version: 0.13.2\n  contact:\n    version: 9.9.9\n\npaths: {}\n';
  assert.equal(versionOf(yaml), '0.13.2');
  assert.equal(versionOf('info:\n  title: X\n'), null);
  assert.equal(versionOf(readFileSync('api/openapi.yaml', 'utf8')), JSON.parse(readFileSync('sdk/package.json', 'utf8')).version);
});

test('bumpsBreaking wants a minor while 0.x and a major from 1.0', () => {
  assert.deepEqual(parseVersion('0.13.2'), [0, 13, 2]);
  assert.equal(bumpsBreaking('0.13.2', '0.14.0'), true);
  assert.equal(bumpsBreaking('0.13.2', '0.13.3'), false);
  assert.equal(bumpsBreaking('0.13.2', '1.0.0'), true);
  assert.equal(bumpsBreaking('1.4.0', '1.5.0'), false);
  assert.equal(bumpsBreaking('1.4.0', '2.0.0'), true);
  assert.equal(bumpsBreaking(null, '2.0.0'), false);
});

const clean = {
  changed: true,
  breaking: false,
  labelled: false,
  release: false,
  baseVersion: '0.13.2',
  headVersion: '0.13.2',
  webVersion: '0.13.2',
};

test('a feature PR: additive passes, breaking needs the label, the version is not checked', () => {
  assert.deepEqual(violationsOf(clean), []);
  assert.match(violationsOf({ ...clean, breaking: true }).join(), /no `contract-breaking` label/);
  assert.deepEqual(violationsOf({ ...clean, breaking: true, labelled: true }), []);
  assert.deepEqual(violationsOf({ ...clean, headVersion: '9.9.9' }), []);
});

test('a release PR: version equals the web, moves when the contract moved, minor for a breaking change', () => {
  const release = { ...clean, release: true, headVersion: '0.13.3', webVersion: '0.13.3' };
  assert.deepEqual(violationsOf(release), []);
  assert.match(violationsOf({ ...release, webVersion: '0.13.4' }).join(), /must equal web\/package.json/);
  assert.match(
    violationsOf({ ...release, headVersion: '0.13.2', webVersion: '0.13.2' }).join(),
    /still 0\.13\.2/,
  );
  assert.deepEqual(violationsOf({ ...release, changed: false, headVersion: '0.13.2', webVersion: '0.13.2' }), []);
  assert.match(violationsOf({ ...release, breaking: true, labelled: true }).join(), /needs a new minor/);
  assert.deepEqual(
    violationsOf({ ...release, breaking: true, labelled: true, headVersion: '0.14.0', webVersion: '0.14.0' }),
    [],
  );
});

test('the committed operation map has exactly one entry per operationId in the contract', async () => {
  const contract = JSON.parse(readFileSync('sdk/openapi.json', 'utf8'));
  const { OPERATIONS, CONTRACT_VERSION } = await import('../../../sdk/index.js');
  const ids = operationsOf(contract).map(({ id }) => id);
  assert.ok(ids.length > 0);
  assert.deepEqual(Object.keys(OPERATIONS), ids);
  for (const { id, method, path } of operationsOf(contract)) assert.deepEqual(OPERATIONS[id], { method, path });
  assert.equal(CONTRACT_VERSION, contract.info.version);
});
