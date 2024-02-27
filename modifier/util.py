import json
import sys

def load():
    path = sys.argv[1]
    with open(path) as f:
        data = json.load(f)
        return data["transactions"], data["beancount_transactions"]

def dump(txns, bc_txns):
    path = sys.argv[1]
    with open(path, 'w') as f:
        json.dump({"transactions": txns, "beancount_transactions": bc_txns}, f)

