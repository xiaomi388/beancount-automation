package persistence

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/plaid/plaid-go/plaid"
	"github.com/xiaomi388/beancount-automation/pkg/types"
)

const schema = `
CREATE TABLE IF NOT EXISTS owners (
    name TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS institutions (
    owner_name   TEXT NOT NULL REFERENCES owners(name),
    name         TEXT NOT NULL,
    type         TEXT NOT NULL CHECK(type IN ('transactions', 'investments')),
    access_token TEXT NOT NULL DEFAULT '',
    cursor       TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (owner_name, name, type)
);

CREATE TABLE IF NOT EXISTS accounts (
    id           TEXT PRIMARY KEY,
    owner_name   TEXT NOT NULL,
    inst_name    TEXT NOT NULL,
    inst_type    TEXT NOT NULL,
    account_base TEXT NOT NULL,
    FOREIGN KEY (owner_name, inst_name, inst_type) REFERENCES institutions(owner_name, name, type)
);

CREATE TABLE IF NOT EXISTS transactions (
    transaction_id TEXT PRIMARY KEY,
    account_id     TEXT NOT NULL REFERENCES accounts(id),
    data           TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS holdings (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id TEXT NOT NULL REFERENCES accounts(id),
    data       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS securities (
    security_id TEXT NOT NULL,
    account_id  TEXT NOT NULL REFERENCES accounts(id),
    data        TEXT NOT NULL,
    PRIMARY KEY (security_id, account_id)
);

CREATE TABLE IF NOT EXISTS investment_transactions (
    transaction_id TEXT PRIMARY KEY,
    account_id     TEXT NOT NULL REFERENCES accounts(id),
    data           TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_institutions_owner ON institutions(owner_name);
CREATE INDEX IF NOT EXISTS idx_accounts_inst ON accounts(owner_name, inst_name, inst_type);
CREATE INDEX IF NOT EXISTS idx_transactions_account ON transactions(account_id);
CREATE INDEX IF NOT EXISTS idx_holdings_account ON holdings(account_id);
CREATE INDEX IF NOT EXISTS idx_securities_account ON securities(account_id);
CREATE INDEX IF NOT EXISTS idx_inv_transactions_account ON investment_transactions(account_id);
`

// Type aliases bypass plaid's custom UnmarshalJSON which silently drops data
// when enum fields (e.g. AccountType) are empty. The aliases use Go's default
// JSON decoder, which populates all fields regardless of enum validation.
type (
	rawAccountBase   plaid.AccountBase
	rawTransaction   plaid.Transaction
	rawHolding       plaid.Holding
	rawSecurity      plaid.Security
	rawInvestmentTxn plaid.InvestmentTransaction
)

func unmarshalAccountBase(data []byte) (plaid.AccountBase, error) {
	var r rawAccountBase
	_ = json.Unmarshal(data, &r)
	return plaid.AccountBase(r), nil
}

func unmarshalTransaction(data []byte) (plaid.Transaction, error) {
	var r rawTransaction
	_ = json.Unmarshal(data, &r)
	return plaid.Transaction(r), nil
}

func unmarshalHolding(data []byte) (plaid.Holding, error) {
	var r rawHolding
	_ = json.Unmarshal(data, &r)
	return plaid.Holding(r), nil
}

func unmarshalSecurity(data []byte) (plaid.Security, error) {
	var r rawSecurity
	_ = json.Unmarshal(data, &r)
	return plaid.Security(r), nil
}

func unmarshalInvestmentTxn(data []byte) (plaid.InvestmentTransaction, error) {
	var r rawInvestmentTxn
	_ = json.Unmarshal(data, &r)
	return plaid.InvestmentTransaction(r), nil
}

// SQLiteStore implements Store using a SQLite database.
type SQLiteStore struct {
	db   *sql.DB
	path string
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return &SQLiteStore{db: db, path: path}, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) LoadOwners() ([]types.Owner, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Load all owners
	ownerRows, err := tx.Query("SELECT name FROM owners ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to query owners: %w", err)
	}
	defer ownerRows.Close()

	var owners []types.Owner
	for ownerRows.Next() {
		var name string
		if err := ownerRows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan owner: %w", err)
		}
		owners = append(owners, types.Owner{Name: name})
	}
	if err := ownerRows.Err(); err != nil {
		return nil, fmt.Errorf("owner rows error: %w", err)
	}

	if owners == nil {
		return []types.Owner{}, nil
	}

	// For each owner, load institutions
	for i := range owners {
		if err := s.loadOwnerData(tx, &owners[i]); err != nil {
			return nil, fmt.Errorf("failed to load data for owner %s: %w", owners[i].Name, err)
		}
	}

	return owners, nil
}

func (s *SQLiteStore) loadOwnerData(tx *sql.Tx, owner *types.Owner) error {
	// Load transaction institutions
	instRows, err := tx.Query(
		"SELECT name, access_token, cursor FROM institutions WHERE owner_name = ? AND type = 'transactions' ORDER BY name",
		owner.Name,
	)
	if err != nil {
		return fmt.Errorf("failed to query transaction institutions: %w", err)
	}
	defer instRows.Close()

	for instRows.Next() {
		var inst types.TransactionInstitution
		if err := instRows.Scan(&inst.InstitutionBase.Name, &inst.InstitutionBase.AccessToken, &inst.InstitutionBase.Cursor); err != nil {
			return fmt.Errorf("failed to scan institution: %w", err)
		}
		inst.TransactionAccounts = []types.TransactionAccount{}

		// Load accounts for this institution
		if err := s.loadTransactionAccounts(tx, owner.Name, inst.InstitutionBase.Name, &inst); err != nil {
			return err
		}

		owner.TransactionInstitutions = append(owner.TransactionInstitutions, inst)
	}
	if err := instRows.Err(); err != nil {
		return fmt.Errorf("institution rows error: %w", err)
	}

	// Load investment institutions
	invInstRows, err := tx.Query(
		"SELECT name, access_token, cursor FROM institutions WHERE owner_name = ? AND type = 'investments' ORDER BY name",
		owner.Name,
	)
	if err != nil {
		return fmt.Errorf("failed to query investment institutions: %w", err)
	}
	defer invInstRows.Close()

	for invInstRows.Next() {
		var inst types.InvestmentInstitution
		inst.InvestmentAccounts = []types.InvestmentAccount{}
		if err := invInstRows.Scan(&inst.InstitutionBase.Name, &inst.InstitutionBase.AccessToken, &inst.InstitutionBase.Cursor); err != nil {
			return fmt.Errorf("failed to scan investment institution: %w", err)
		}

		if err := s.loadInvestmentAccounts(tx, owner.Name, inst.InstitutionBase.Name, &inst); err != nil {
			return err
		}

		owner.InvestmentInstitutions = append(owner.InvestmentInstitutions, inst)
	}
	if err := invInstRows.Err(); err != nil {
		return fmt.Errorf("investment institution rows error: %w", err)
	}

	return nil
}

func (s *SQLiteStore) loadTransactionAccounts(tx *sql.Tx, ownerName, instName string, inst *types.TransactionInstitution) error {
	acctRows, err := tx.Query(
		"SELECT id, account_base FROM accounts WHERE owner_name = ? AND inst_name = ? AND inst_type = 'transactions' ORDER BY id",
		ownerName, instName,
	)
	if err != nil {
		return fmt.Errorf("failed to query accounts: %w", err)
	}
	defer acctRows.Close()

	for acctRows.Next() {
		var acctID, acctBaseJSON string
		if err := acctRows.Scan(&acctID, &acctBaseJSON); err != nil {
			return fmt.Errorf("failed to scan account: %w", err)
		}

		acctBase, err := unmarshalAccountBase([]byte(acctBaseJSON))
		if err != nil {
			return fmt.Errorf("failed to unmarshal account base: %w", err)
		}

		acct := types.TransactionAccount{
			AccoutBase:   acctBase,
			Transactions: map[string]plaid.Transaction{},
		}

		// Load transactions
		txnRows, err := tx.Query("SELECT transaction_id, data FROM transactions WHERE account_id = ?", acctID)
		if err != nil {
			return fmt.Errorf("failed to query transactions: %w", err)
		}

		for txnRows.Next() {
			var txnID, txnData string
			if err := txnRows.Scan(&txnID, &txnData); err != nil {
				txnRows.Close()
				return fmt.Errorf("failed to scan transaction: %w", err)
			}
			txn, err := unmarshalTransaction([]byte(txnData))
			if err != nil {
				txnRows.Close()
				return fmt.Errorf("failed to unmarshal transaction: %w", err)
			}
			acct.Transactions[txnID] = txn
		}
		txnRows.Close()
		if err := txnRows.Err(); err != nil {
			return fmt.Errorf("transaction rows error: %w", err)
		}

		inst.TransactionAccounts = append(inst.TransactionAccounts, acct)
	}
	return acctRows.Err()
}

func (s *SQLiteStore) loadInvestmentAccounts(tx *sql.Tx, ownerName, instName string, inst *types.InvestmentInstitution) error {
	acctRows, err := tx.Query(
		"SELECT id, account_base FROM accounts WHERE owner_name = ? AND inst_name = ? AND inst_type = 'investments' ORDER BY id",
		ownerName, instName,
	)
	if err != nil {
		return fmt.Errorf("failed to query investment accounts: %w", err)
	}
	defer acctRows.Close()

	for acctRows.Next() {
		var acctID, acctBaseJSON string
		if err := acctRows.Scan(&acctID, &acctBaseJSON); err != nil {
			return fmt.Errorf("failed to scan account: %w", err)
		}

		acctBase, err := unmarshalAccountBase([]byte(acctBaseJSON))
		if err != nil {
			return fmt.Errorf("failed to unmarshal account base: %w", err)
		}

		acct := types.InvestmentAccount{
			AccoutBase:   acctBase,
			Holdings:     []plaid.Holding{},
			Securities:   map[string]plaid.Security{},
			Transactions: map[string]plaid.InvestmentTransaction{},
		}

		// Load holdings
		holdRows, err := tx.Query("SELECT data FROM holdings WHERE account_id = ?", acctID)
		if err != nil {
			return fmt.Errorf("failed to query holdings: %w", err)
		}
		for holdRows.Next() {
			var data string
			if err := holdRows.Scan(&data); err != nil {
				holdRows.Close()
				return fmt.Errorf("failed to scan holding: %w", err)
			}
			h, err := unmarshalHolding([]byte(data))
			if err != nil {
				holdRows.Close()
				return fmt.Errorf("failed to unmarshal holding: %w", err)
			}
			acct.Holdings = append(acct.Holdings, h)
		}
		holdRows.Close()

		// Load securities
		secRows, err := tx.Query("SELECT security_id, data FROM securities WHERE account_id = ?", acctID)
		if err != nil {
			return fmt.Errorf("failed to query securities: %w", err)
		}
		for secRows.Next() {
			var secID, data string
			if err := secRows.Scan(&secID, &data); err != nil {
				secRows.Close()
				return fmt.Errorf("failed to scan security: %w", err)
			}
			sec, err := unmarshalSecurity([]byte(data))
			if err != nil {
				secRows.Close()
				return fmt.Errorf("failed to unmarshal security: %w", err)
			}
			acct.Securities[secID] = sec
		}
		secRows.Close()

		// Load investment transactions
		invTxnRows, err := tx.Query("SELECT transaction_id, data FROM investment_transactions WHERE account_id = ?", acctID)
		if err != nil {
			return fmt.Errorf("failed to query investment transactions: %w", err)
		}
		for invTxnRows.Next() {
			var txnID, data string
			if err := invTxnRows.Scan(&txnID, &data); err != nil {
				invTxnRows.Close()
				return fmt.Errorf("failed to scan investment transaction: %w", err)
			}
			txn, err := unmarshalInvestmentTxn([]byte(data))
			if err != nil {
				invTxnRows.Close()
				return fmt.Errorf("failed to unmarshal investment transaction: %w", err)
			}
			acct.Transactions[txnID] = txn
		}
		invTxnRows.Close()

		inst.InvestmentAccounts = append(inst.InvestmentAccounts, acct)
	}
	return acctRows.Err()
}

func (s *SQLiteStore) DumpOwners(owners []types.Owner) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear all data in reverse dependency order
	for _, table := range []string{"investment_transactions", "securities", "holdings", "transactions", "accounts", "institutions", "owners"} {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return fmt.Errorf("failed to clear table %s: %w", table, err)
		}
	}

	for _, owner := range owners {
		if _, err := tx.Exec("INSERT INTO owners (name) VALUES (?)", owner.Name); err != nil {
			return fmt.Errorf("failed to insert owner %s: %w", owner.Name, err)
		}

		// Insert transaction institutions
		for _, inst := range owner.TransactionInstitutions {
			if _, err := tx.Exec(
				"INSERT INTO institutions (owner_name, name, type, access_token, cursor) VALUES (?, ?, 'transactions', ?, ?)",
				owner.Name, inst.InstitutionBase.Name, inst.InstitutionBase.AccessToken, inst.InstitutionBase.Cursor,
			); err != nil {
				return fmt.Errorf("failed to insert transaction institution: %w", err)
			}

			for _, acct := range inst.TransactionAccounts {
				acctBaseJSON, err := json.Marshal(acct.AccoutBase)
				if err != nil {
					return fmt.Errorf("failed to marshal account base: %w", err)
				}

				if _, err := tx.Exec(
					"INSERT INTO accounts (id, owner_name, inst_name, inst_type, account_base) VALUES (?, ?, ?, 'transactions', ?)",
					acct.AccoutBase.AccountId, owner.Name, inst.InstitutionBase.Name, string(acctBaseJSON),
				); err != nil {
					return fmt.Errorf("failed to insert account: %w", err)
				}

				for txnID, txn := range acct.Transactions {
					txnData, err := json.Marshal(txn)
					if err != nil {
						return fmt.Errorf("failed to marshal transaction: %w", err)
					}
					if _, err := tx.Exec(
						"INSERT INTO transactions (transaction_id, account_id, data) VALUES (?, ?, ?)",
						txnID, acct.AccoutBase.AccountId, string(txnData),
					); err != nil {
						return fmt.Errorf("failed to insert transaction: %w", err)
					}
				}
			}
		}

		// Insert investment institutions
		for _, inst := range owner.InvestmentInstitutions {
			if _, err := tx.Exec(
				"INSERT INTO institutions (owner_name, name, type, access_token, cursor) VALUES (?, ?, 'investments', ?, ?)",
				owner.Name, inst.InstitutionBase.Name, inst.InstitutionBase.AccessToken, inst.InstitutionBase.Cursor,
			); err != nil {
				return fmt.Errorf("failed to insert investment institution: %w", err)
			}

			for _, acct := range inst.InvestmentAccounts {
				acctBaseJSON, err := json.Marshal(acct.AccoutBase)
				if err != nil {
					return fmt.Errorf("failed to marshal account base: %w", err)
				}

				if _, err := tx.Exec(
					"INSERT INTO accounts (id, owner_name, inst_name, inst_type, account_base) VALUES (?, ?, ?, 'investments', ?)",
					acct.AccoutBase.AccountId, owner.Name, inst.InstitutionBase.Name, string(acctBaseJSON),
				); err != nil {
					return fmt.Errorf("failed to insert investment account: %w", err)
				}

				for _, h := range acct.Holdings {
					data, err := json.Marshal(h)
					if err != nil {
						return fmt.Errorf("failed to marshal holding: %w", err)
					}
					if _, err := tx.Exec(
						"INSERT INTO holdings (account_id, data) VALUES (?, ?)",
						acct.AccoutBase.AccountId, string(data),
					); err != nil {
						return fmt.Errorf("failed to insert holding: %w", err)
					}
				}

				for secID, sec := range acct.Securities {
					data, err := json.Marshal(sec)
					if err != nil {
						return fmt.Errorf("failed to marshal security: %w", err)
					}
					if _, err := tx.Exec(
						"INSERT INTO securities (security_id, account_id, data) VALUES (?, ?, ?)",
						secID, acct.AccoutBase.AccountId, string(data),
					); err != nil {
						return fmt.Errorf("failed to insert security: %w", err)
					}
				}

				for txnID, txn := range acct.Transactions {
					data, err := json.Marshal(txn)
					if err != nil {
						return fmt.Errorf("failed to marshal investment transaction: %w", err)
					}
					if _, err := tx.Exec(
						"INSERT INTO investment_transactions (transaction_id, account_id, data) VALUES (?, ?, ?)",
						txnID, acct.AccoutBase.AccountId, string(data),
					); err != nil {
						return fmt.Errorf("failed to insert investment transaction: %w", err)
					}
				}
			}
		}
	}

	return tx.Commit()
}
