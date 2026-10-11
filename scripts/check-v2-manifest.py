#!/usr/bin/env python3
"""Validate the public load manifest, independently of seed generation."""
import json, sys
from datetime import datetime, timezone, timedelta
manifest = json.load(open(sys.argv[1]))
assert manifest['version'] == 1 and manifest['mode'] == 'v2'
assert len(manifest['products']) == 30000
accounts = {account['id']: account for account in manifest['accounts']}
assert len(accounts) == 10000
assert sum(account['completedSales'] for account in accounts.values()) == 180000
assert len({product['productId'] for product in manifest['products']}) == 30000
assert len({product['sellerId'] for product in manifest['products']}) == 50
assert len({product['idempotencyKey'] for product in manifest['products']}) == 30000
for product in manifest['products']:
    assert product['sellerId'] in accounts and product['buyerId'] in accounts
    assert product['sellerId'] != product['buyerId']
    assert product['priceCents'] > 0
    assert datetime.fromisoformat(product['expiresAt'].replace('Z','+00:00')) > datetime.now(timezone.utc)+timedelta(hours=24)
print('Manifest: 30000 unique dedicated products, 10000 registered accounts, legal sellers/buyers, valid prices/expiry, baseline sales 180000')
