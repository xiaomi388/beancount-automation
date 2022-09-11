from collections import defaultdict
from itertools import zip_longest
import re

def gen_from_account(txn: dict) -> str:
    owner = txn["owner"]
    name =  re.sub(r'[^a-zA-Z]', '', txn["account"]["name"])
    plaid_typ = txn["account"]["type"].capitalize()
    bean_typ = "Assets" if plaid_typ in ("Depository", "Other") else "Liabilities"
    return f"{bean_typ}:{owner}:US:{txn['institution']}:{plaid_typ}:{name}"

def gen_to_account(txn: dict) -> str:
    owner = txn["owner"]
    category = re.sub(r'[^a-zA-Z:]', '', ':'.join(txn["category"]))
    bean_typ = "Expenses" if txn["amount"] > 0 else "Income"
    return f"{bean_typ}:{owner}:US:{category}"

def merge_transfer(txn_args: list) -> list:
    """ merge transfer transactions based on the amount"""
    amount_to_args = defaultdict(list)
    for arg in txn_args:
        amount_to_args[arg["amount"]].append(arg)

    merged_args = []
    other_args = []
    for amount in amount_to_args:
        arg1s = amount_to_args[amount]
        if -amount in amount_to_args:
            arg2s = amount_to_args[-amount]
            while arg1s and arg2s:
                arg1, arg2 = arg1s.pop(), arg2s.pop()
                if arg1["from_account"] == arg2["from_account"]:
                    other_args += [arg1, arg2]
                    continue
                merged_args.append({
                    "date": max(arg1["date"], arg2["date"]),
                    "desc": '"Transfer Gened"',
                    "from_account": arg1["from_account"],
                    "to_account": arg2["from_account"],
                    "unit": arg2["unit"],
                    "amount": arg1["amount"],
                })
        other_args += arg1s
    return merged_args + other_args





