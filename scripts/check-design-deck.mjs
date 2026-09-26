import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';

const deck = process.argv[2] ?? 'docs/design/deck';
const manifestName = 'MANIFEST.sha256';

function filesUnder(dir) {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    return statSync(path).isDirectory() ? filesUnder(path) : [path];
  });
}

function sha256(path) {
  return createHash('sha256').update(readFileSync(path)).digest('hex');
}

function localReferences(html) {
  const found = new Set();
  const attribute = /()\b(?:src|href)="([^"{}]+)"/g;
  const literal = /(sample\s*:\s*)?['"]([^'"{}\n]+?\.(?:svg|dc\.html|js))(?:#[^'"]*)?['"]/g;
  for (const pattern of [attribute, literal]) {
    for (const [, fixture, target] of html.matchAll(pattern)) {
      if (fixture || /^(?:[a-z]+:|\/\/|#)/i.test(target)) continue;
      found.add(target.split('#')[0].replace(/^\.\//, ''));
    }
  }
  return found;
}

try {
  const manifestPath = join(deck, manifestName);
  assert.ok(existsSync(manifestPath), `${manifestPath} is missing`);

  const listed = new Map(
    readFileSync(manifestPath, 'utf8')
      .split('\n')
      .filter(Boolean)
      .map((line) => {
        const match = /^([0-9a-f]{64}) {2}(.+)$/.exec(line);
        assert.ok(match, `Malformed manifest line: ${line}`);
        return [match[2], match[1]];
      }),
  );

  const present = filesUnder(deck)
    .map((path) => relative(deck, path))
    .filter((path) => path !== manifestName && path !== 'README.md');

  for (const path of present) {
    assert.ok(listed.has(path), `${path} is in the deck but not in ${manifestName}`);
    assert.equal(sha256(join(deck, path)), listed.get(path), `${path} differs from ${manifestName}`);
  }
  for (const path of listed.keys()) {
    assert.ok(present.includes(path), `${manifestName} lists ${path}, which is missing`);
  }

  const pages = present.filter((path) => path.endsWith('.dc.html'));
  assert.ok(pages.length > 0, 'The deck has no .dc.html pages');
  for (const page of pages) {
    const html = readFileSync(join(deck, page), 'utf8');
    assert.equal(html.match(/<x-dc>/g)?.length, 1, `${page} must hold exactly one <x-dc> template`);
    assert.equal(html.match(/data-dc-script/g)?.length, 1, `${page} must hold exactly one data-dc-script block`);
    for (const target of localReferences(html)) {
      assert.ok(existsSync(join(deck, target)), `${page} references ${target}, which is not in the deck`);
    }
  }

  console.log(`Deck OK: ${present.length} files, ${pages.length} pages`);
} catch (error) {
  console.error(error.message);
  process.exit(1);
}
