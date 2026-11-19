#!/usr/bin/env python3
"""Fill prices.csv with closing prices for stock plan (RSU) vest dates.

`bean-auto dump` uses prices.csv as the cost basis (fair market value) of
shares deposited into Plaid "stock plan" accounts. Plaid does not report the
FMV, so this script estimates it with the daily close from Yahoo Finance's
chart endpoint on the deposit date (or the previous trading day).

Rows whose source is not "yahoo-close" (for example values typed in from a
Schwab Equity Awards export or a 1099-B supplement) are never overwritten.

Usage: fetch_prices.py OWNERS_JSON PRICES_CSV
"""
import csv
import json
import subprocess
import sys
import time
import urllib.error
import urllib.request
from datetime import date, datetime, timedelta, timezone

SOURCE = "yahoo-close"
UA = {"User-Agent": "Mozilla/5.0"}


def stock_plan_needs(owners):
    """Return {symbol: set(dates)} for vests and the opening date of each account."""
    needs = {}
    for owner in owners:
        for inst in owner.get("investmentInstitutions", []):
            for acct in inst.get("InvestmentAccounts", []):
                base = acct["accountBase"]
                if base.get("subtype") != "stock plan":
                    continue
                secs = acct.get("securities") or {}
                own = [t for t in (acct.get("transactions") or {}).values()
                       if t["account_id"] == base["account_id"]]
                for t in own:
                    sec = secs.get(t.get("security_id") or "", {})
                    sym = (sec.get("ticker_symbol") or "").upper()
                    if not sym or sym.startswith("CUR:") or sec.get("type") == "cash":
                        continue
                    vest = t["quantity"] > 0 and t["amount"] == 0
                    trade = t["type"] in ("buy", "sell", "transfer")
                    if vest or trade:
                        needs.setdefault(sym, set()).add(t["date"])
                if own:
                    first = min(t["date"] for t in own)
                    opening = (date.fromisoformat(first) - timedelta(days=1)).isoformat()
                    for h in acct.get("holdings") or []:
                        sym = (secs.get(h["security_id"], {}).get("ticker_symbol") or "").upper()
                        if sym and not sym.startswith("CUR:"):
                            needs.setdefault(sym, set()).add(opening)
    return needs


def http_get(url):
    """GET with urllib; fall back to curl when Python has no CA bundle
    (common with python.org builds on macOS)."""
    try:
        with urllib.request.urlopen(urllib.request.Request(url, headers=UA), timeout=30) as r:
            return r.read()
    except urllib.error.URLError as e:
        if "CERTIFICATE_VERIFY_FAILED" not in str(e):
            raise
        return subprocess.run(["curl", "-fsS", "-m", "30", "-A", UA["User-Agent"], url],
                              check=True, capture_output=True).stdout


def fetch_closes(symbol, start, end):
    p1 = int(datetime.combine(start - timedelta(days=10), datetime.min.time(), timezone.utc).timestamp())
    p2 = int(datetime.combine(end + timedelta(days=2), datetime.min.time(), timezone.utc).timestamp())
    url = (f"https://query1.finance.yahoo.com/v8/finance/chart/{symbol}"
           f"?period1={p1}&period2={p2}&interval=1d")
    data = json.loads(http_get(url))["chart"]["result"][0]
    tz = data["meta"].get("gmtoffset", 0)
    closes = {}
    for ts, c in zip(data["timestamp"], data["indicators"]["quote"][0]["close"]):
        if c is None:
            continue
        d = datetime.fromtimestamp(ts + tz, timezone.utc).date().isoformat()
        closes[d] = round(c, 4)
    return closes


def main():
    if len(sys.argv) != 3:
        print(__doc__)
        return 2
    owners_path, prices_path = sys.argv[1], sys.argv[2]
    owners = json.load(open(owners_path))
    needs = stock_plan_needs(owners)

    rows = {}
    try:
        with open(prices_path, newline="") as f:
            for rec in csv.reader(f):
                if len(rec) < 3 or rec[0].startswith("#") or rec[0] == "date":
                    continue
                rows[(rec[0], rec[1].upper())] = rec + [""] * (4 - len(rec))
    except FileNotFoundError:
        pass

    added = kept = 0
    for sym, dates in sorted(needs.items()):
        ds = sorted(dates)
        closes = fetch_closes(sym, date.fromisoformat(ds[0]), date.fromisoformat(ds[-1]))
        trading = sorted(closes)
        for d in ds:
            key = (d, sym)
            if key in rows and rows[key][3] != SOURCE:
                kept += 1
                continue
            prior = [t for t in trading if t <= d]
            if not prior:
                print(f"no close for {sym} on or before {d}", file=sys.stderr)
                continue
            rows[key] = [d, sym, str(closes[prior[-1]]), SOURCE]
            added += 1
        time.sleep(0.5)

    with open(prices_path, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["date", "symbol", "price", "source"])
        for key in sorted(rows, key=lambda k: (k[1], k[0])):
            w.writerow(rows[key][:4])
    print(f"{prices_path}: {added} estimated rows written, {kept} manual rows kept")
    return 0


if __name__ == "__main__":
    sys.exit(main())
