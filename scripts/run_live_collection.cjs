// Called by the integration-tagged Go test against its live loopback API.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const newman = require('newman');

const collection = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
const expected = JSON.parse(fs.readFileSync(process.argv[3], 'utf8'));
const seen = new Set();
const failures = [];
newman.run({ collection, reporters: [], timeoutRequest: 5000, timeout: 45000 }, (error, summary) => {
  if (error) failures.push(error.message);
  if (summary) {
    for (const failure of summary.run.failures) failures.push(failure.error.message);
  }
  for (const name of Object.keys(expected)) {
    if (!seen.has(name)) failures.push(`Request was not executed: ${name}`);
  }
  if (failures.length) {
    console.error(failures.join('\n'));
    process.exitCode = 1;
  } else {
    console.log(`PASS: ${seen.size} generated requests executed against the live Chi API; all statuses and JSON responses matched.`);
  }
}).on('request', (error, args) => {
  const name = args.item.name;
  try {
    if (error) throw error;
    assert.ok(Object.hasOwn(expected, name), `Unexpected request: ${name}`);
    assert.ok(!seen.has(name), `Duplicate request: ${name}`);
    seen.add(name);
    assert.equal(args.response.code, expected[name].status, `${name}: HTTP status`);
    assert.deepEqual(args.response.json(), expected[name].json, `${name}: response JSON`);
  } catch (failure) {
    failures.push(`${name}: ${failure.message}`);
  }
});
