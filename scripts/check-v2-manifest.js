// Read-only independent manifest/database correspondence evidence. Manifest is
// loaded by the shell as manifestJSON; it is never executed as source text.
const manifest = JSON.parse(manifestJSON);
if (manifest.database !== db.getName()) throw new Error("manifest database mismatch");
const accounts = new Map(manifest.accounts.map(a=>[a.id,a]));
let violations=0;
for (let offset=0;offset<manifest.products.length;offset+=1000) {
  const rows=manifest.products.slice(offset,offset+1000);
  const products=new Map(db.products.find({_id:{$in:rows.map(p=>p.productId)}}).toArray().map(p=>[p._id,p]));
  for (const row of rows) {
    const product=products.get(row.productId);
    if (!product || product.seedCohort!=="dedicated" || product.sold || product.sellerId!==row.sellerId || Number(product.priceCents)!==row.priceCents || product.expiresAt.getTime()!==new Date(row.expiresAt).getTime() || row.buyerId===row.sellerId || !accounts.has(row.buyerId)) violations++;
  }
}
for (const user of db.users.find().toArray()) {
  const account=accounts.get(user._id);
  if (!account || account.username!==user.username || account.completedSales!==user.completedSales) violations++;
}
if (violations) throw new Error(`manifest/database violations: ${violations}`);
print("Manifest/database correspondence: 30000 designated products and 10000 accounts passed");
