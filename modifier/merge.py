#!/usr/bin/env python3
"""Merge beancount transactions by collapsing self and inter transfers."""

import util


def account_to_string(account):
    """Return a stable identifier for an account mapping."""

    return (
        f'{account["type"]}:{account["owner"]}:'
        f'{account["country"]}:{account["institution"]}:'
        f'{account["plaid_account_type"]}:{account["name"]}'
    )


def create_merged_transaction(from_txn, to_txn, description):
    """Build a merged transaction that matches the legacy script output."""

    payer = from_txn["from_account"]["owner"]
    payee = to_txn["to_account"]["owner"]

    return {
        "date": max(from_txn["date"], to_txn["date"]),
        "payee": payee,
        "desc": description,
        "from_account": from_txn["from_account"],
        "to_account": to_txn["to_account"],
        "unit": from_txn["unit"],
        "metadata": {
            "payer": payer,
            "from_id": from_txn["metadata"]["id"],
            "to_id": to_txn["metadata"]["id"],
        },
        "tags": [],
        "amount": from_txn["amount"],
    }


def merge_self_transfers(transactions):
    merged_transactions = []
    processed_indices = set()

    for i, from_txn in enumerate(transactions):
        if i in processed_indices or from_txn["from_account"]["type"] != "Assets":
            continue

        for j, to_txn in enumerate(transactions):
            if (
                j in processed_indices
                or j == i
                or from_txn["unit"] != to_txn["unit"]
                or from_txn["amount"] != to_txn["amount"]
            ):
                continue

            if to_txn["to_account"]["type"] not in ("Assets", "Liabilities"):
                continue

            if from_txn["from_account"]["owner"] != to_txn["to_account"]["owner"]:
                continue

            if account_to_string(from_txn["from_account"]) == account_to_string(
                to_txn["to_account"]
            ):
                continue

            merged_txn = create_merged_transaction(from_txn, to_txn, "self transfer")
            merged_transactions.append(merged_txn)
            processed_indices.update([i, j])
            break

    return merged_transactions, processed_indices


def merge_inter_transfers(transactions, processed_indices):
    merged_transactions = []

    for i, from_txn in enumerate(transactions):
        if i in processed_indices or from_txn["from_account"]["type"] != "Assets":
            continue

        for j, to_txn in enumerate(transactions):
            if (
                j in processed_indices
                or j == i
                or from_txn["unit"] != to_txn["unit"]
                or to_txn["to_account"]["type"] != "Assets"
                or from_txn["amount"] != to_txn["amount"]
                or to_txn["to_account"]["owner"] == from_txn["from_account"]["owner"]
            ):
                continue

            if from_txn["date"] != to_txn["date"]:
                continue

            payer = from_txn["from_account"]["owner"]
            payee = to_txn["to_account"]["owner"]
            description = f"transfer {payer} -> {payee}"

            merged_txn = create_merged_transaction(from_txn, to_txn, description)
            merged_transactions.append(merged_txn)
            processed_indices.update([i, j])
            break

    return merged_transactions, processed_indices


def append_unprocessed(transactions, processed_indices, merged_transactions):
    final_transactions = list(merged_transactions)

    for i, txn in enumerate(transactions):
        if i not in processed_indices:
            final_transactions.append(txn)

    return final_transactions


def merge_transactions(transactions):
    self_transfers, processed_indices = merge_self_transfers(transactions)
    inter_transfers, processed_indices = merge_inter_transfers(transactions, processed_indices)
    merged = self_transfers + inter_transfers
    return append_unprocessed(transactions, processed_indices, merged)


def main():
    _, bc_txns = util.load()
    util.dump(merge_transactions(bc_txns))


if __name__ == "__main__":
    main()
