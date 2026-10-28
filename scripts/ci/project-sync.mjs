import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

/**
 * project-sync keeps the org GitHub Project in step with this repository's
 * issues and pull requests. Project threads cannot reach org Projects, but
 * they can label issues and write pull request bodies, so those are the
 * interface: a label `status:in-progress`, `next:owner`, `priority:p1`,
 * `release:v0.14.0` or `workstream:product` sets that field on the issue's
 * item, a `Thread:` or `Plan ID:` line in a body sets that text field, and a
 * pull request whose body says `Task: #N` (or `Closes #N`) moves issue N to
 * In Progress while it is open and to Done when it merges. `Part of: #N`
 * links without finishing. Issues are never closed here.
 */

export const LABEL_FIELDS = {
  status: 'Status',
  next: 'Next step by',
  priority: 'Priority',
  release: 'Release',
  workstream: 'Workstream',
};

export const TEXT_FIELDS = { thread: 'Thread', 'plan id': 'Plan ID' };

export function slug(text) {
  return String(text)
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '');
}

/**
 * option finds the single-select option a label value names, by its whole name
 * or its first words: "p1" names "P1 high". A value two options start with
 * names neither.
 */
export function option(options, value) {
  const wanted = slug(value);
  if (!wanted) return undefined;
  const exact = options.find((o) => slug(o.name) === wanted);
  if (exact) return exact;
  const starts = options.filter((o) => slug(o.name).startsWith(`${wanted}-`));
  return starts.length === 1 ? starts[0] : undefined;
}

export function fieldsFromLabels(labels) {
  const fields = {};
  for (const label of labels ?? []) {
    const name = typeof label === 'string' ? label : label.name;
    const at = name.indexOf(':');
    if (at < 0) continue;
    const field = LABEL_FIELDS[name.slice(0, at).trim().toLowerCase()];
    if (field) fields[field] = name.slice(at + 1).trim();
  }
  return fields;
}

export function fieldsFromBody(body) {
  const fields = {};
  for (const line of String(body ?? '').split(/\r?\n/)) {
    const match = /^\s*(thread|plan id)\s*:\s*(.+?)\s*$/i.exec(line);
    if (match) fields[TEXT_FIELDS[match[1].toLowerCase()]] = match[2];
  }
  return fields;
}

/** taskRefs reads the issues a pull request body names: finishes on merge, or only links. */
export function taskRefs(body) {
  const finishes = new Set();
  const links = new Set();
  for (const line of String(body ?? '').split(/\r?\n/)) {
    const match = /^\s*[-*]?\s*(tasks?|closes|fixes|resolves|part of)\s*:?\s+((?:#\d+[\s,]*(?:and\s+)?)+)/i.exec(line);
    if (!match) continue;
    const target = /^part of$/i.test(match[1]) ? links : finishes;
    for (const n of match[2].matchAll(/#(\d+)/g)) target.add(Number(n[1]));
  }
  for (const n of finishes) links.delete(n);
  return { finishes: [...finishes], links: [...links] };
}

/** Pull requests from these authors, or from a branch of this repository, may move issues. */
export const TRUSTED = new Set(['OWNER', 'MEMBER', 'COLLABORATOR']);

/**
 * plan turns one webhook event into the field values to write, one entry per
 * issue. Each event writes only what it changed: a new label sets its own
 * field, an edit only the body's text fields, so a stale label never undoes
 * what a pull request moved. Fields not named are left as they are; a removed
 * label clears nothing.
 */
export function plan(eventName, event) {
  if (eventName === 'issues') {
    const issue = event.issue;
    if (!issue || issue.pull_request) return [];
    let fields;
    if (event.action === 'labeled') fields = fieldsFromLabels(event.label ? [event.label] : []);
    else if (event.action === 'edited') fields = fieldsFromBody(issue.body);
    else if (event.action === 'closed') fields = { Status: 'Done' };
    else if (event.action === 'opened' || event.action === 'reopened') {
      fields = { ...fieldsFromBody(issue.body), ...fieldsFromLabels(issue.labels) };
      fields.Status ??= 'Todo';
    } else return [];
    return Object.keys(fields).length ? [{ issue: issue.number, fields }] : [];
  }
  if (eventName === 'pull_request' || eventName === 'pull_request_target') {
    const pr = event.pull_request;
    if (!pr) return [];
    const sameRepo = pr.head?.repo?.full_name && pr.head.repo.full_name === pr.base?.repo?.full_name;
    if (!sameRepo && !TRUSTED.has(pr.author_association)) return [];
    const { finishes, links } = taskRefs(pr.body);
    const text = fieldsFromBody(pr.body);
    const out = [];
    if (event.action === 'closed') {
      if (!pr.merged) return [];
      for (const n of finishes) out.push({ issue: n, fields: { ...text, Status: 'Done' } });
      return out;
    }
    for (const n of [...finishes, ...links]) out.push({ issue: n, fields: { ...text, Status: 'In Progress' } });
    return out;
  }
  return [];
}

async function graphql(token, query, variables) {
  const response = await fetch(process.env.GITHUB_GRAPHQL_URL || 'https://api.github.com/graphql', {
    method: 'POST',
    headers: { authorization: `bearer ${token}`, 'content-type': 'application/json' },
    body: JSON.stringify({ query, variables }),
  });
  const json = await response.json();
  if (!response.ok || json.errors) throw new Error(JSON.stringify(json.errors ?? json));
  return json.data;
}

const PROJECT = `query($owner: String!, $number: Int!) {
  organization(login: $owner) { projectV2(number: $number) { id fields(first: 50) { nodes {
    ... on ProjectV2FieldCommon { id name dataType }
    ... on ProjectV2SingleSelectField { options { id name } }
  } } } }
}`;
const ISSUE = `query($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) { issue(number: $number) { id } }
}`;
const ADD = `mutation($project: ID!, $content: ID!) {
  addProjectV2ItemById(input: { projectId: $project, contentId: $content }) { item { id } }
}`;
const SET = `mutation($project: ID!, $item: ID!, $field: ID!, $value: ProjectV2FieldValue!) {
  updateProjectV2ItemFieldValue(input: { projectId: $project, itemId: $item, fieldId: $field, value: $value }) { projectV2Item { id } }
}`;

async function main() {
  const token = process.env.PROJECT_TOKEN;
  if (!token) {
    console.log('::notice::PROJECT_TOKEN is not set, so the project is not updated.');
    return;
  }
  const event = JSON.parse(readFileSync(process.env.GITHUB_EVENT_PATH, 'utf8'));
  const writes = plan(process.env.GITHUB_EVENT_NAME, event);
  if (!writes.length) return;

  const owner = process.env.PROJECT_OWNER || 'Quizzivy';
  const number = Number(process.env.PROJECT_NUMBER || 2);
  const [repoOwner, repo] = process.env.GITHUB_REPOSITORY.split('/');
  const project = (await graphql(token, PROJECT, { owner, number })).organization.projectV2;
  const fields = new Map(project.fields.nodes.filter((f) => f.name).map((f) => [f.name, f]));

  let failed = 0;
  for (const write of writes) {
    try {
      await apply(token, project, fields, repoOwner, repo, write);
    } catch (error) {
      failed++;
      console.log(`::error::#${write.issue}: ${error.message}`);
    }
  }
  if (failed) process.exitCode = 1;
}

async function apply(token, project, fields, repoOwner, repo, write) {
  const issue = (await graphql(token, ISSUE, { owner: repoOwner, repo, number: write.issue })).repository.issue;
  if (!issue) throw new Error(`not an issue in ${repoOwner}/${repo}`);
  const item = (await graphql(token, ADD, { project: project.id, content: issue.id })).addProjectV2ItemById.item;
  for (const [name, value] of Object.entries(write.fields)) {
    const field = fields.get(name);
    if (!field) {
      console.log(`::warning::the project has no field "${name}"`);
      continue;
    }
    let input;
    if (field.options) {
      const chosen = option(field.options, value);
      if (!chosen) {
        console.log(`::warning::"${value}" is not an option of ${name}`);
        continue;
      }
      input = { singleSelectOptionId: chosen.id };
    } else {
      input = { text: value };
    }
    await graphql(token, SET, { project: project.id, item: item.id, field: field.id, value: input });
    console.log(`#${write.issue}: ${name} = ${value}`);
  }
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.log(`::error::${error.message}`);
    process.exitCode = 1;
  });
}
