import { appendFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

/**
 * verdict compares what each job did with what the plan asked of it and
 * returns the reasons the run is not green. A planned job must have succeeded,
 * a job the plan skipped must have been skipped, and the two lists must name
 * the same jobs, so a job added to the workflow without a plan entry fails.
 */
export function verdict(needs, planned) {
  if (needs.plan?.result !== 'success' || !planned) {
    return [`plan: ${needs.plan?.result ?? 'missing'}, so nothing can be trusted`];
  }
  const failures = [];
  const ran = Object.keys(needs).filter((id) => id !== 'plan');
  for (const id of ran) {
    if (!planned.jobs[id]) failures.push(`${id}: in the workflow but not in scripts/ci/plan.mjs`);
  }
  for (const [id, job] of Object.entries(planned.jobs)) {
    const result = needs[id]?.result;
    const expected = job.run ? 'success' : 'skipped';
    if (result === undefined) failures.push(`${id}: in scripts/ci/plan.mjs but the gate does not wait for it`);
    else if (result !== expected) failures.push(`${id}: ${result}, expected ${expected}`);
  }
  return failures;
}

function main() {
  const needs = JSON.parse(process.env.NEEDS);
  const planned = needs.plan?.outputs?.plan ? JSON.parse(needs.plan.outputs.plan) : null;
  const failures = verdict(needs, planned);
  const lines = failures.length ? ['### CI failed', '', ...failures.map((f) => `- ${f}`)] : ['### CI passed'];
  if (process.env.GITHUB_STEP_SUMMARY) appendFileSync(process.env.GITHUB_STEP_SUMMARY, lines.join('\n') + '\n');
  for (const failure of failures) console.log(`::error::${failure}`);
  process.exitCode = failures.length ? 1 : 0;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) main();
