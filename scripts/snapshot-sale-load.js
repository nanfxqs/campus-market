// Read-only full dedicated cohort evidence. The enclosing runner supplies a
// JSON manifest through a copied file, never executable interpolated data.
const manifest=JSON.parse(require('fs').readFileSync('/tmp/v2-load-manifest.json','utf8'));
if(manifest.database!==db.getName()) throw new Error('manifest/database mismatch');
const products={};
const successRecords={};
const listings=new Set();
for(let offset=0;offset<manifest.products.length;offset+=1000){
  const ids=manifest.products.slice(offset,offset+1000).map(p=>p.productId);
  db.products.find({_id:{$in:ids}},{sellerId:1,sold:1,sale:1,expiresAt:1,seedCohort:1}).forEach(p=>products[p._id]={sellerId:p.sellerId,sold:p.sold,sale:p.sale ? {buyerId:p.sale.buyerId,priceCents:Number(p.sale.priceCents),confirmedAt:p.sale.confirmedAt.toISOString()}:null,expiresAt:p.expiresAt.toISOString(),seedCohort:p.seedCohort});
  db.listings.find({_id:{$in:ids}},{_id:1}).forEach(p=>listings.add(p._id));
  db.transactions.find({productId:{$in:ids},success:true}).forEach(t=>{
    (successRecords[t.productId] ||= []).push({id:t._id,productId:t.productId,sellerId:t.sellerId,buyerId:t.buyerId,priceCents:Number(t.priceCents),idempotencyKey:t.idempotencyKey,success:t.success,code:t.code,confirmedAt:t.confirmedAt.toISOString()});
  });
}
for(const id of Object.keys(products)) products[id].listing=listings.has(id);
const sellerCounts={};
db.users.find({},{_id:1,completedSales:1}).forEach(u=>sellerCounts[u._id]=Number(u.completedSales));
print(JSON.stringify({schemaVersion:1,database:db.getName(),snapshotTime:new Date().toISOString(),metadata:db.seed_metadata.findOne({_id:'dataset'}),counts:{users:db.users.countDocuments({}),products:db.products.countDocuments({}),sold:db.products.countDocuments({sold:true}),listings:db.listings.countDocuments({}),transactions:db.transactions.countDocuments({}),successfulTransactions:db.transactions.countDocuments({success:true}),priceChanges:db.priceChanges.countDocuments({})},products,successRecords,sellerCounts}));
