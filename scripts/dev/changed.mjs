#!/usr/bin/env node
// Runs a command on what changed since BASE (default origin/work/redesign-r4),
// measured from the merge base so work merged to BASE since the branch point is not counted.
//   changed.mjs ts,tsx -- eslint   appends the changed web files with those extensions
//   changed.mjs since -- vitest run --changed   appends the merge-base revision
import { execFileSync, spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { extname, relative, resolve } from 'node:path';

const DEFAULT_BASE = 'origin/work/redesign-r4';

const separator = process.argv.indexOf('--');
if (separator < 3 || separator === process.argv.length - 1) {
  console.error('usage: changed.mjs <ext,ext,...|since> -- <command> [args...]');
  process.exit(2);
}
const mode = process.argv[2];
const [command, ...args] = process.argv.slice(separator + 1);

const git = (...gitArgs) => execFileSync('git', gitArgs, { encoding: 'utf8' }).split('\n').filter(Boolean);

const base = process.env.BASE || DEFAULT_BASE;
const root = git('rev-parse', '--show-toplevel')[0];
const mergeBase = git('merge-base', base, 'HEAD')[0];

function run(extra) {
  const result = spawnSync(command, [...args, ...extra], { stdio: 'inherit' });
  process.exit(result.status ?? 1);
}

if (mode === 'since') {
  run([mergeBase]);
}

const extensions = new Set(mode.split(',').map((ext) => `.${ext}`));
const changed = new Set([
  ...git('diff', '--name-only', '--diff-filter=ACMR', mergeBase),
  ...git('ls-files', '--others', '--exclude-standard'),
]);

const web = resolve(root, 'web');
const files = [...changed]
  .map((path) => resolve(root, path))
  .filter((path) => path.startsWith(`${web}/`) && extensions.has(extname(path)) && existsSync(path))
  .map((path) => relative(process.cwd(), path))
  .sort();

if (files.length === 0) {
  console.log(`no changed files against ${base} (${mergeBase.slice(0, 8)})`);
  process.exit(0);
}

console.log(`${files.length} changed file(s) against ${base} (${mergeBase.slice(0, 8)})`);
run(files);
