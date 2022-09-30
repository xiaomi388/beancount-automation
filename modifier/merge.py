#!/usr/bin/env python3
import util

txns, bc_txns = util.load()

new_txns, new_bc_txns = [], []

merged_idx = set()

for i in range(len(txns)):
    if bc_txns[i]["from_account"]["type"] != "Assets":
        continue
    if i in merged_idx: 
        continue
    for q in range(len(txns)):
        if q == i or bc_txns[i]["unit"] != bc_txns[q]["unit"] or bc_txns[q]["to_account"]["type"] not in ("Assets", "Liabilities") or bc_txns[i]["amount"] != bc_txns[q]["amount"] or bc_txns[q]["to_account"] == bc_txns[i]["from_account"]:
            continue

        # one can only pay for its own liabilities
        if bc_txns[q]["to_account"]["type"] == "Liabilities" and bc_txns[i]["from_account"]["owner"] != bc_txns[q]["to_account"]["owner"]:
            continue

        new_txn = {}
        payer = bc_txns[i]["from_account"]["owner"]
        payee = bc_txns[q]["to_account"]["owner"]
        new_bc_txn = {
            "date": max(bc_txns[i]["date"], bc_txns[q]["date"]),
            "payee": payee,
            "desc": f"transfer {payer} -> {payee}",
            "from_account": bc_txns[i]["from_account"],
            "to_account": bc_txns[q]["to_account"],
            "unit": bc_txns[i]["unit"],
            "metadata": {
                "payer": payer,
            },
            "tags": [],
            "amount": bc_txns[i]["amount"]
        }

        new_txns.append(new_txn)
        new_bc_txns.append(new_bc_txn)
        merged_idx.update([i, q])
        break


for i in range(len(txns)):
    if i not in merged_idx:
        new_txns.append(txns[i])
        new_bc_txns.append(bc_txns[i])

util.dump(new_txns, new_bc_txns)

