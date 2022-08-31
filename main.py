import plaid
from plaid.api import plaid_api
from plaid.model.link_token_create_request import LinkTokenCreateRequest
from plaid.model.link_token_create_request_user import LinkTokenCreateRequestUser
from plaid.model.item_public_token_exchange_request import ItemPublicTokenExchangeRequest
from plaid.model.products import Products
from plaid.model.country_code import CountryCode
from plaid.model.transactions_sync_request import TransactionsSyncRequest
import datetime
import tempfile
import pickledb
import os.path
import os
import secret

root_path = __file__
db = pickledb.load(os.path.join("plaid.db"), auto_dump=True)

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
    print(response)
    access_token = response['access_token']
    return access_token

def link():
    link_token = create_link_token()
    generate_auth_page(link_token)
    public_token = input("input public token:")
    access_token = exchange_public_token(public_token)
    db.set("access_token", access_token)
    print(access_token)

def get_transactions():
    access_token = db.get("access_token")
    cursor = db.get("next_cursor")
    if cursor == False:
        cursor = None

    has_more = True
    while has_more:
      request = TransactionsSyncRequest(
        access_token=access_token,
      )
      response = client.transactions_sync(request)
      #TODO: keep fetching transactions and store them into db
      print(response)
      break

def main():
    if not db.exists("access_token"):
        link()
    else:
        get_transactions()

if __name__ == "__main__":
    main()


