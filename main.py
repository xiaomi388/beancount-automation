import json
from plaid.model.transactions_sync_request import TransactionsSyncRequest
import os.path
import os
import click
import typing
import jinja2
import util
import shutil
import plaid_util
import re
import db
import env
from hooks.setting import pre_gen_args_hooks, post_gen_args_hooks

plaid_gen_dir = os.path.join(env.runpath, "plaid_gen") 

@click.command()
@click.option("--owner", help="owner of the account", required=True)
@click.option("--institution", help="name of the account institution", required=True)
def link(owner, institution):
    link_token = plaid_util.create_link_token()
    plaid_util.generate_auth_page(link_token)
    public_token = input("input public token:")
    access_token = plaid_util.exchange_public_token(public_token)

    k = "owners"
    if not db.meta.exists(k):
        db.meta.set(k, list())
    s = set(typing.cast(list, db.meta.get(k)))
    s.add(owner)
    db.meta.set(k, list(s))

    k = f"{owner}:institutions"
    if not db.meta.exists(k):
        db.meta.set(k, {})
    m = typing.cast(dict, db.meta.get(k))
    m[institution] = (access_token, None)
    db.meta.set(k, m)

    db.meta.dump()


@click.command()
def sync():
    owners = typing.cast(list, db.meta.get("owners"))
    if not owners:
        return

    k = "transactions"
    if not db.txn.exists(k):
        db.txn.set("transactions", dict())
    txns = typing.cast(dict, db.txn.get("transactions"))

    for owner in owners:
        institutions = typing.cast(dict, db.meta.get(f"{owner}:institutions"))
        for name in institutions:
            print(f"syncing {owner}:{name}")
            access_token, cursor = institutions[name]
            has_more = True
            while has_more:
                req = TransactionsSyncRequest(
                  access_token=access_token,
                  cursor=cursor if cursor else "",
                )
                resp = plaid_util.client.transactions_sync(req)
                for txn in resp["added"] + resp["modified"]:
                    txn = json.loads(json.dumps(txn.to_dict(), default=str))
                    txn["owner"] = owner
                    txn["account"] = plaid_util.get_account(txn["account_id"])
                    txn["institution"] = name
                    txns[txn["transaction_id"]] = txn
                for txn in resp["removed"]:
                    txns.pop(txn["transaction_id"])
                has_more = resp["has_more"]
                cursor = resp["next_cursor"]

                db.txn.set("transactions", txns)
                db.txn.dump()
                institutions[name][1] = cursor
                db.meta.set(f"{owner}:institutions", institutions)
                db.meta.dump()

def _dump():
    os.mkdir(plaid_gen_dir)
    txn_file_path = os.path.join(plaid_gen_dir, "transaction.beancount")
    main_file_path = os.path.join(plaid_gen_dir, "main.beancount")
    bean_accounts = set()

    txns = list(typing.cast(dict, db.txn.get("transactions")).values())

    for hook in pre_gen_args_hooks:
        txns = hook(txns)

    args = []
    for txn in txns:
        from_account = util.gen_from_account(txn)
        to_account = util.gen_to_account(txn)
        bean_accounts.update([from_account, to_account])
        args.append({
            "institution": txn["institution"],
            "date": txn["date"],
            "desc": f'"{re.sub(r"[^a-zA-Z ]", "", txn["name"])}"',
            "from_account":util.gen_from_account(txn),
            "to_account":util.gen_to_account(txn),
            "amount":txn["amount"],
            "unit":txn["iso_currency_code"],
            "tag": "",
        })

    for hook in post_gen_args_hooks:
        args = hook(args, txns)

    with open(txn_file_path, "w") as f: 
        tpl = jinja2.Environment(loader=jinja2.FileSystemLoader(os.path.join(env.runpath, "templates"))).get_template("transaction.tpl")
        output = tpl.render(txns=args)
        f.write(output) 

    with open(main_file_path, "w") as f:
        f.write('include "transaction.beancount"\n\n')
        for account in bean_accounts:
            f.write(f"2000-01-01 open {account}\n")


@click.command()
def dump():
    backup_dir = None
    if os.path.exists(plaid_gen_dir):
        backup_dir = plaid_gen_dir + ".bak"
        os.rename(plaid_gen_dir, backup_dir)
    try:
        _dump()
    except Exception as e:
        shutil.rmtree(plaid_gen_dir)
        if backup_dir:
            os.rename(backup_dir, plaid_gen_dir)
        raise
    if backup_dir:
        shutil.rmtree(backup_dir)

@click.group()
def cli():
    pass


if __name__ == "__main__":
    cli.add_command(link)
    cli.add_command(sync)
    cli.add_command(dump)
    cli()


