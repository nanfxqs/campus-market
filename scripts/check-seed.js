// Run with mongosh against an explicitly selected seed database; read-only.
const metadata = db.seed_metadata.findOne({_id: "dataset"});
if (!metadata) throw new Error("No completed seed dataset metadata");
const expected = metadata.counts;
function check(name, actual, wanted) {
  print(`${name}: ${actual} (expected ${wanted})`);
  if (actual !== wanted) throw new Error(`${name} mismatch`);
}
function count(collection, pipeline) {
  const result = db[collection].aggregate([...pipeline, {$count: "n"}],
    {allowDiskUse: true}).toArray();
  return result.length ? result[0].n : 0;
}
check("users", db.users.countDocuments({}), expected.users);
check("unsold", db.products.countDocuments({sold: false}), expected.active);
check("sold", db.products.countDocuments({sold: true}), expected.sold);
check("listings", db.listings.countDocuments({}), expected.active);
check("transactions", db.transactions.countDocuments({}), expected.transactions);
check("priceChanges", db.priceChanges.countDocuments({}), expected.priceChanges);
check("successes", db.transactions.countDocuments({success: true}), expected.sold);
check("image violations", db.products.countDocuments({
  $expr: {$or: [{$lt: [{$size: "$images"}, 1]}, {$gt: [{$size: "$images"}, 9]}]}
}), 0);
check("price chain violations", count("priceChanges", [
  {$sort: {productId: 1, changedAt: 1, _id: 1}},
  {$group: {_id: "$productId", history: {$push: {old: "$oldPriceCents", next: "$newPriceCents"}}}},
  {$lookup: {from: "products", localField: "_id", foreignField: "_id", as: "product"}},
  {$unwind: "$product"},
  {$match: {$expr: {$or: [
    {$ne: [{$arrayElemAt: ["$history.old", 0]}, "$product.initialPriceCents"]},
    {$ne: [{$arrayElemAt: ["$history.next", -1]}, "$product.priceCents"]},
    {$anyElementTrue: {$map: {
      input: {$range: [1, {$size: "$history"}]}, as: "i",
      in: {$ne: [
        {$arrayElemAt: ["$history.old", "$$i"]},
        {$arrayElemAt: ["$history.next", {$subtract: ["$$i", 1]}]}
      ]}
    }}}
  ]}}}
]), 0);
check("sale record violations", count("transactions", [
  {$match: {success: true}},
  {$lookup: {from: "products", localField: "productId", foreignField: "_id", as: "product"}},
  {$unwind: {path: "$product", preserveNullAndEmptyArrays: true}},
  {$match: {$expr: {$or: [
    {$ne: ["$product.sold", true]},
    {$ne: ["$sellerId", "$product.sellerId"]},
    {$ne: ["$buyerId", "$product.sale.buyerId"]},
    {$ne: ["$priceCents", "$product.sale.priceCents"]},
    {$ne: ["$confirmedAt", "$product.sale.confirmedAt"]}
  ]}}}
]), 0);
check("seller count violations", count("products", [
  {$match: {sold: true}},
  {$group: {_id: "$sellerId", n: {$sum: 1}}},
  {$lookup: {from: "users", localField: "_id", foreignField: "_id", as: "seller"}},
  {$unwind: {path: "$seller", preserveNullAndEmptyArrays: true}},
  {$match: {$expr: {$ne: ["$n", "$seller.completedSales"]}}}
]), 0);
printjson(db.priceChanges.aggregate([
  {$group: {_id: "$productId", n: {$sum: 1}}},
  {$group: {_id: null, min: {$min: "$n"}, max: {$max: "$n"}, mean: {$avg: "$n"}}}
]).toArray());
print("Seed observations passed. Listing counts are wall-clock sensitive; see seed documentation.");
