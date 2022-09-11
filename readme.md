# How to Run

1. Add `secret.py` and fill in `client_id=xxx` and `secret=xxx`
2. Link an institution to plaid. Owner name and institution name are customized input, which are used for generating beancount output.

`python3 main.py link --owner <owner name of the account> --institution <institution name>`


3. Sync all bank data and save all data to `transaction.db`

`python3 main.py sync`


4. dump data in the format of beancount

`python3 main.py dump`

