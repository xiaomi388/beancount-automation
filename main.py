import plaid
import json
from plaid.api import plaid_api
from plaid.model.link_token_create_request import LinkTokenCreateRequest
from plaid.model.link_token_create_request_user import LinkTokenCreateRequestUser
from plaid.model.item_public_token_exchange_request import ItemPublicTokenExchangeRequest
from plaid.model.products import Products
from plaid.model.country_code import CountryCode
from plaid.model.transactions_sync_request import TransactionsSyncRequest
from plaid.model.accounts_get_request import AccountsGetRequest
import datetime
import tempfile
import pickledb
import os.path
import os
import secret
import click
import typing
import jinja2
import util
import shutil
import re

runpath = os.path.dirname(os.path.realpath(__file__))
plaid_gen_dir = os.path.join(runpath, "plaid_gen") 
metadata_db = pickledb.load(os.path.join(runpath, "metadata.db"), auto_dump=False)
txn_db = pickledb.load(os.path.join(runpath, "transaction.db"), auto_dump=False)

configuration = plaid.Configuration(
    host=plaid.Environment.Development,
    api_key={
        'clientId': secret.client_id,
        'secret': secret.secret,
    }
)
api_client = plaid.ApiClient(configuration)
client = plaid_api.PlaidApi(api_client)

def generate_auth_page(link_token):
    page = """<html>
    <body>
    <button id='linkButton'>Open Link - Institution Select</button>
    <p id="results"></p>
    <script src="https://cdn.plaid.com/link/v2/stable/link-initialize.js"></script>
    <script>
    var linkHandler = Plaid.create({
    token: '""" + link_token + """',
    onLoad: function() {
    // The Link module finished loading.
    },
    onSuccess: function(public_token, metadata) {
    // Send the public_token to your app server here.
    // The metadata object contains info about the institution the
    // user selected and the account ID, if selectAccount is enabled.
    console.log('public_token: '+public_token+', metadata: '+JSON.stringify(metadata));
    document.getElementById("results").innerHTML = "public_token: " + public_token + "<br>metadata: " + metadata;
    },
    onExit: function(err, metadata) {
    // The user exited the Link flow.
    if (err != null) {
    // The user encountered a Plaid API error prior to exiting.
    }
    // metadata contains information about the institution
    // that the user selected and the most recent API request IDs.
    // Storing this information can be helpful for support.
    }
    });
    // Trigger the standard institution select view
    document.getElementById('linkButton').onclick = function() {
    linkHandler.open();
    };
    </script>
    </body>
    </html>
    """
    with tempfile.NamedTemporaryFile(suffix=".html", delete=False) as tmp:
        tmp.write(str.encode(page))
        os.system(f"open {tmp.name}")

def create_link_token():
    # Get the client_user_id by searching for the current user
    # Create a link_token for the given user
    request = LinkTokenCreateRequest(
            products=[Products("auth")],
            client_name="Plaid Test App",
            country_codes=[CountryCode('US')],
            language='en',
            user=LinkTokenCreateRequestUser(
                client_user_id="123456"
            )
        )
    response = client.link_token_create(request)
    # Send the data to the client
    return response["link_token"]

def exchange_public_token(public_token):
    request = ItemPublicTokenExchangeRequest(
      public_token=public_token
    )
    response = client.item_public_token_exchange(request)
    access_token = response['access_token']
    return access_token

@click.command()
@click.option("--owner", help="owner of the account", required=True)
@click.option("--institution", help="name of the account institution", required=True)
def link(owner, institution):
    link_token = create_link_token()
    generate_auth_page(link_token)
    public_token = input("input public token:")
    access_token = exchange_public_token(public_token)

    k = "owners"
    if not metadata_db.exists(k):
        metadata_db.set(k, list())
    s = set(typing.cast(list, metadata_db.get(k)))
    s.add(owner)
    metadata_db.set(k, list(s))

    k = f"{owner}:institutions"
    if not metadata_db.exists(k):
        metadata_db.set(k, [])
    m = typing.cast(dict, metadata_db.get(k))
    m[institution] = (access_token, None)
    metadata_db.set(k, m)

    metadata_db.dump()

def get_account(account_id):
    if not get_account.accounts:
        owners = typing.cast(list, metadata_db.get("owners"))
        if not owners:
            return
        for owner in owners:
            institutions = typing.cast(dict, metadata_db.get(f"{owner}:institutions"))
            for name in institutions:
                access_token, _ = institutions[name]
                req = AccountsGetRequest(access_token=access_token)
                resp = client.accounts_get(req)
                get_account.accounts += resp["accounts"]

    for account in get_account.accounts:
        if account["account_id"] == account_id: 
           account = json.loads(json.dumps(account.to_dict(), default=str))
           account = {k: account[k] for k in ("name", "type")} 
           return account
    raise Exception(f"account {account_id} not found")
get_account.accounts = []


@click.command()
def sync():
    owners = typing.cast(list, metadata_db.get("owners"))
    if not owners:
        return

    k = "transactions"
    if not txn_db.exists(k):
        txn_db.set("transactions", dict())
    txns = typing.cast(dict, txn_db.get("transactions"))

    for owner in owners:
        institutions = typing.cast(dict, metadata_db.get(f"{owner}:institutions"))
        for name in institutions:
            access_token, cursor = institutions[name]
            has_more = True
            while has_more:
                req = TransactionsSyncRequest(
                  access_token=access_token,
                  cursor=cursor if cursor else "",
                )
                resp = client.transactions_sync(req)
                for txn in resp["added"] + resp["modified"]:
                    txn = json.loads(json.dumps(txn.to_dict(), default=str))
                    txn["owner"] = owner
                    txn["account"] = get_account(txn["account_id"])
                    txn["institution"] = name
                    txns[txn["transaction_id"]] = txn
                for id in resp["removed"]:
                    txns.pop(id)
                has_more = resp["has_more"]
                cursor = resp["next_cursor"]

                txn_db.set("transactions", txns)
                txn_db.dump()
                institutions[name][1] = cursor
                metadata_db.set(f"{owner}:institutions", institutions)
                metadata_db.dump()

def _dump():
    os.mkdir(plaid_gen_dir)
    txn_file_path = os.path.join(plaid_gen_dir, "transaction.beancount")
    main_file_path = os.path.join(plaid_gen_dir, "main.beancount")
    bean_accounts = set()

    txns = typing.cast(dict, txn_db.get("transactions"))
    args = []
    for txn in txns.values():
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
        })
    args = util.merge_transfer(args)

    with open(txn_file_path, "w") as f: 
        tpl = jinja2.Environment(loader=jinja2.FileSystemLoader(os.path.join(runpath, "templates"))).get_template("transaction.tpl")
        for arg in args:
            output = tpl.render(**arg)
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


