#!/usr/bin/env python3
"""Merge history of re-linked Plaid Items in owners.yaml (JSON).

When an Item is removed and linked again, Plaid issues new account_ids, so
`bean-auto sync` adds new account records next to the old ones. This script,
run once after the first sync of a re-linked Item, matches old and new
accounts by mask within the same owner/institution and:

  transactions accounts: keeps every NEW transaction, adds OLD transactions
    dated before the earliest NEW one (no overlap, no duplicates), then drops
    the old record.
  investment accounts: drops the old record (the new Item re-pulls holdings
    and up to 24 months of investment transactions).

Dry-run by default; pass --apply to write. Writes a timestamped backup first.
Usage: merge_relinked.py OWNERS_JSON [--only Owner:Institution ...] [--apply]
"""
import argparse
import json
import shutil
import sys
from datetime import datetime


def merge_txn_inst(inst, live_ids):
    """Return (accounts, report). live_ids: account_ids returned by Plaid now."""
    accts = inst["transactionAccounts"]
    new = [a for a in accts if a["accountBase"]["account_id"] in live_ids]
    old = [a for a in accts if a["accountBase"]["account_id"] not in live_ids]
    report, keep_old = [], []
    for o in old:
        mask = o["accountBase"].get("mask")
        match = [n for n in new if n["accountBase"].get("mask") == mask]
        if len(match) != 1:
            keep_old.append(o)
            report.append(f"  keep old *{mask}: {len(match)} new matches, left untouched")
            continue
        n = match[0]
        n_txns = n.get("transactions") or {}
        o_txns = o.get("transactions") or {}
        if not n_txns:
            keep_old.append(o)
            report.append(f"  keep old *{mask}: new account has no transactions yet, sync first")
            continue
        cutoff = min(t["date"] for t in n_txns.values())
        moved = {k: t for k, t in o_txns.items() if t["date"] < cutoff and k not in n_txns}
        dropped = len(o_txns) - len(moved)
        n_txns.update(moved)
        n["transactions"] = n_txns
        report.append(
            f"  *{mask}: new {len(n_txns) - len(moved)} txns from {cutoff}; "
            f"kept {len(moved)} older txns; dropped {dropped} overlapping old txns"
        )
    return new + keep_old, report


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("owners")
    ap.add_argument("--only", nargs="*", default=None, help="Owner:Institution filters")
    ap.add_argument("--live-ids", required=True, help="JSON file: {\"Owner:Inst\": [account_id, ...]} from Plaid")
    ap.add_argument("--apply", action="store_true")
    args = ap.parse_args()

    data = json.load(open(args.owners))
    live = json.load(open(args.live_ids))
    only = set(args.only) if args.only else None
    changed = False

    for owner in data:
        for inst in owner.get("transactionInstitutions", []):
            key = f"{owner['name']}:{inst['institutionBase']['name']}"
            if (only and key not in only) or key not in live:
                continue
            print(key)
            accts, report = merge_txn_inst(inst, set(live[key]))
            print("\n".join(report) or "  nothing to merge")
            if len(accts) != len(inst["transactionAccounts"]) or report:
                inst["transactionAccounts"] = accts
                changed = True
        for inst in owner.get("investmentInstitutions", []):
            key = f"{owner['name']}:{inst['InstitutionBase']['name']}"
            if (only and key not in only) or key not in live:
                continue
            ids = set(live[key])
            new = [a for a in inst["InvestmentAccounts"] if a["accountBase"]["account_id"] in ids]
            old = [a for a in inst["InvestmentAccounts"] if a["accountBase"]["account_id"] not in ids]
            new_masks = {a["accountBase"].get("mask") for a in new}
            drop = [a for a in old if a["accountBase"].get("mask") in new_masks]
            print(key)
            for a in drop:
                print(f"  drop old investment *{a['accountBase'].get('mask')} (superseded by new Item)")
            if drop:
                inst["InvestmentAccounts"] = [a for a in inst["InvestmentAccounts"] if a not in drop]
                changed = True

    if not args.apply:
        print("\nDRY RUN, nothing written. Re-run with --apply.")
        return
    if changed:
        bak = f"{args.owners}.bak.{datetime.now():%Y%m%d_%H%M%S}"
        shutil.copy2(args.owners, bak)
        json.dump(data, open(args.owners, "w"), indent=2, ensure_ascii=False)
        print(f"\nWritten. Backup: {bak}")
    else:
        print("\nNo changes.")


if __name__ == "__main__":
    sys.exit(main())
