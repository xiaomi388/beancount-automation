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





