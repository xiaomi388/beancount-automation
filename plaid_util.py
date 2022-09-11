import plaid
import tempfile
import secret
import os
import typing
import json
from plaid.api import plaid_api
from plaid.model.link_token_create_request import LinkTokenCreateRequest
from plaid.model.link_token_create_request_user import LinkTokenCreateRequestUser
from plaid.model.item_public_token_exchange_request import ItemPublicTokenExchangeRequest
from plaid.model.accounts_get_request import AccountsGetRequest
from plaid.model.products import Products
from plaid.model.country_code import CountryCode
import db

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
            products=[Products("transactions")],
            client_name="Beancount Automation",
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

def get_account(account_id):
    if not get_account.accounts:
        owners = typing.cast(list, db.meta.get("owners"))
        if not owners:
            return
        for owner in owners:
            institutions = typing.cast(dict, db.meta.get(f"{owner}:institutions"))
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

