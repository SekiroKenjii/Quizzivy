#!/usr/bin/env node
// The SDK gate (D-080), run by `make sdk-check` locally and in CI's Contract job.
//
// It diffs api/openapi.yaml against the same file at the merge-base with
// SDK_BASE, never the base tip: changes landed on the base since the branch
// point would otherwise read as removals. oasdiff reports breaking changes
// (ERR) and changes a client may trip on (WARN); the changelog and both go to
// the job summary.
//
// Environment:
//   SDK_BASE           the ref to diff against (default origin/develop)
//   SDK_GATE           report (default) prints findings and exits 0; block fails on them
//   CONTRACT_BREAKING  true when the PR carries the contract-breaking label
//   SDK_RELEASE        true on a PR into main: the release rules apply too
//   OASDIFF            the oasdiff command (the Makefile passes a pinned `go run`)
import { execFileSync, spawnSync } from 'node:child_process';
import { appendFileSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const CONTRACT = 'api/openapi.yaml';
const IGNORE = 'api/oasdiff-ignore.txt';
const LABEL = 'contract-breaking';

/** parseVersion reads MAJOR.MINOR.PATCH, or returns null. */
export function parseVersion(text) {
  const match = /^(\d+)\.(\d+)\.(\d+)$/.exec(text ?? '');
  return match ? match.slice(1).map(Number) : null;
}

/**
 * bumpsBreaking reports whether head is a version a breaking change may ship
 * in after base: a new minor while 0.x, a new major from 1.0. A patch never
 * may, so a hotfix cannot carry a breaking change.
 */
export function bumpsBreaking(base, head) {
  const [bMajor, bMinor] = parseVersion(base) ?? [];
  const [hMajor, hMinor] = parseVersion(head) ?? [];
  if (hMajor === undefined || bMajor === undefined) return false;
  if (bMajor === 0 && hMajor === 0) return hMinor > bMinor;
  return hMajor > bMajor;
}

/** versionOf reads info.version from the contract's YAML without a parser. */
export function versionOf(yaml) {
  const info = /^info:\n((?:[ \t]+.*\n|\n)*)/m.exec(yaml);
  const version = info && /^ {2}version:\s*["']?([^"'\s#]+)/m.exec(info[1]);
  return version ? version[1] : null;
}

/**
 * violationsOf applies the D-080 rules to one diff and returns what breaks
 * them, as sentences. `breaking` is whether oasdiff found an ERR.
 */
export function violationsOf({ changed, breaking, labelled, release, baseVersion, headVersion, webVersion }) {
  const out = [];
  if (breaking && !labelled) {
    out.push(
      `the contract has a breaking change and the PR has no \`${LABEL}\` label. Make it additive (expand, then contract in a later release), or add the label, cite the D-entry or the owner's approval in the PR body, and add the release-notes line for tabs loaded before the deploy.`,
    );
  }
  if (!release) return out;
  if (headVersion !== webVersion) {
    out.push(`info.version (${headVersion}) must equal web/package.json's version (${webVersion}) in a release.`);
  }
  if (changed && headVersion === baseVersion) {
    out.push(`the contract changed but info.version is still ${baseVersion}, the version already published from main.`);
  }
  if (breaking && !bumpsBreaking(baseVersion, headVersion)) {
    out.push(
      `a breaking change needs a new minor while 0.x (a new major from 1.0): ${baseVersion} to ${headVersion} is not enough. A patch hotfix cannot carry one.`,
    );
  }
  return out;
}

function git(...args) {
  return execFileSync('git', args, { encoding: 'utf8', maxBuffer: 64 << 20 });
}

function oasdiff(command, args) {
  const [bin, ...pre] = command.split(/\s+/).filter(Boolean);
  const result = spawnSync(bin, [...pre, ...args], { encoding: 'utf8', maxBuffer: 64 << 20 });
  if (result.error) throw result.error;
  return result;
}

function main() {
  const base = process.env.SDK_BASE || 'origin/develop';
  const block = process.env.SDK_GATE === 'block';
  const labelled = process.env.CONTRACT_BREAKING === 'true';
  const release = process.env.SDK_RELEASE === 'true';
  const command = process.env.OASDIFF || 'oasdiff';
  const summary = process.env.GITHUB_STEP_SUMMARY;
  const lines = [];
  const say = (text = '') => {
    console.log(text);
    lines.push(text);
  };

  let mergeBase;
  try {
    mergeBase = git('merge-base', base, 'HEAD').trim();
  } catch {
    throw new Error(`no merge-base with ${base}. Fetch it first (git fetch origin <branch>) or set SDK_BASE.`);
  }
  const head = readFileSync(CONTRACT, 'utf8');
  const baseText = git('show', `${mergeBase}:${CONTRACT}`);
  const headVersion = versionOf(head);
  const baseVersion = versionOf(baseText);
  const webVersion = JSON.parse(readFileSync('web/package.json', 'utf8')).version;
  const changed = head !== baseText;

  say(`## SDK contract check`);
  say();
  say(`Against ${base} at merge-base \`${mergeBase.slice(0, 12)}\`: info.version ${baseVersion} to ${headVersion}.`);
  say();

  let breaking = false;
  if (changed) {
    const dir = mkdtempSync(join(tmpdir(), 'sdk-check-'));
    try {
      const basePath = join(dir, 'base.yaml');
      writeFileSync(basePath, baseText);
      const ignore = ['--err-ignore', IGNORE, '--warn-ignore', IGNORE];
      const found = oasdiff(command, ['breaking', basePath, CONTRACT, '--fail-on', 'ERR', '--format', 'markdown', ...ignore]);
      if (found.status !== 0 && found.status !== 1) {
        process.stderr.write(found.stderr);
        throw new Error(`oasdiff breaking exited ${found.status}`);
      }
      breaking = found.status === 1;
      say(`### Breaking changes and warnings`);
      say();
      say(found.stdout.trim() || 'None.');
      say();
      const log = oasdiff(command, ['changelog', basePath, CONTRACT, '--format', 'markdown']);
      if (log.status !== 0) {
        process.stderr.write(log.stderr);
        throw new Error(`oasdiff changelog exited ${log.status}`);
      }
      say(`<details><summary>Changelog</summary>`);
      say();
      say(log.stdout.trim() || 'No changes oasdiff can see.');
      say();
      say(`</details>`);
      say();
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  } else {
    say('The contract is unchanged since the merge-base.');
    say();
  }

  const violations = violationsOf({ changed, breaking, labelled, release, baseVersion, headVersion, webVersion });
  if (violations.length === 0) {
    say(breaking ? `Breaking, and labelled \`${LABEL}\`.` : 'Nothing breaks the SDK rules.');
  } else {
    say(block ? '**Blocked:**' : '**Would block** (the gate is report-only for now, D-080):');
    for (const violation of violations) {
      say(`- ${violation}`);
      console.log(`::${block ? 'error' : 'warning'} title=SDK contract check::${violation}`);
    }
  }
  if (summary) appendFileSync(summary, `${lines.join('\n')}\n`);
  if (block && violations.length > 0) process.exit(1);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try {
    main();
  } catch (error) {
    // While the gate is report-only, a tool that will not run must not turn
    // the branch red either; it shows as a warning on the run instead.
    if (process.env.SDK_GATE === 'block') throw error;
    console.log(`::warning title=SDK contract check::could not run: ${error.message}`);
    console.error(`sdk-check: ${error.message}`);
  }
}
