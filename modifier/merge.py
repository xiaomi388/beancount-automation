#!/usr/bin/env python3
import util

_, bc_txns = util.load()

new_bc_txns = []

merged_idx = set()

def account_to_string(account):
    return f'{account["type"]}:{account["owner"]}:{account["country"]}:{account["institution"]}:{account["plaid_account_type"]}:{account["name"]}'

# self transfer
for i in range(len(bc_txns)):
    if bc_txns[i]["from_account"]["type"] != "Assets":
        continue
    if i in merged_idx:
        continue

    for q in range(len(bc_txns)):
        if q in merged_idx or q == i or bc_txns[i]["unit"] != bc_txns[q]["unit"] or bc_txns[i]["amount"] != bc_txns[q]["amount"]:
            continue

        if bc_txns[q]["to_account"]["type"] not in ("Assets", "Liabilities"):
            continue

        if bc_txns[i]["from_account"]["owner"] != bc_txns[q]["to_account"]["owner"]:
            continue

        if account_to_string(bc_txns[i]["from_account"]) == account_to_string(bc_txns[q]["to_account"]):
            continue


        payer = bc_txns[i]["from_account"]["owner"]
        payee = bc_txns[q]["to_account"]["owner"]
        new_txn = {}
        new_bc_txn = {
            "date": max(bc_txns[i]["date"], bc_txns[q]["date"]),
            "payee": payee,
            "desc": f"self transfer",
            "from_account": bc_txns[i]["from_account"],
            "to_account": bc_txns[q]["to_account"],
            "unit": bc_txns[i]["unit"],
            "metadata": {
                "payer": payer,
                "from_id": bc_txns[i]["metadata"]["id"],
                "to_id": bc_txns[q]["metadata"]["id"]
            },
            "tags": [],
            "amount": bc_txns[i]["amount"]
        }

        new_bc_txns.append(new_bc_txn)
        merged_idx.update([i, q])
        break


# inter transfer
for i in range(len(bc_txns)):
    if bc_txns[i]["from_account"]["type"] != "Assets":
        continue
    if i in merged_idx: 
        continue
    for q in range(len(bc_txns)):
        if q in merged_idx:
            continue
        if q == i or bc_txns[i]["unit"] != bc_txns[q]["unit"] or bc_txns[q]["to_account"]["type"] != "Assets" or bc_txns[i]["amount"] != bc_txns[q]["amount"] or bc_txns[q]["to_account"]["owner"] == bc_txns[i]["from_account"]["owner"]:
            continue

        if bc_txns[i]["date"] != bc_txns[q]["date"]:
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
                "from_id": bc_txns[i]["metadata"]["id"],
                "to_id": bc_txns[q]["metadata"]["id"]
            },
            "tags": [],
            "amount": bc_txns[i]["amount"]
        }

        new_bc_txns.append(new_bc_txn)
        merged_idx.update([i, q])
        break


for i in range(len(bc_txns)):
    if i not in merged_idx:
        new_bc_txns.append(bc_txns[i])

util.dump(new_bc_txns)
