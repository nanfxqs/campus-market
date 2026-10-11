// Read-only logical document fingerprints; field order is not data identity.
const crypto = require('crypto');
function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === 'object') return Object.fromEntries(Object.keys(value).sort().map(key=>[key,canonical(value[key])]));
  return value;
}
for (const collection of ['users','products','listings','transactions','priceChanges','profileChanges','categoryChanges','seed_metadata']) {
  const digest = crypto.createHash('sha256');
  db[collection].find().sort({_id:1}).forEach(doc=>digest.update(JSON.stringify(canonical(EJSON.serialize(doc)))+'\n'));
  print(`${collection} ${digest.digest('hex')}`);
}
