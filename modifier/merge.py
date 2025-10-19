#!/usr/bin/env python3
"""
Transaction Merger for Beancount Transactions

This script processes beancount transactions to identify and merge related transactions:
1. Self-transfers: Transactions between accounts owned by the same person
2. Inter-transfers: Transactions between different people on the same date

The script helps consolidate duplicate or related transactions that represent the same
financial event but are recorded separately in different accounts.
"""

import util


def account_to_string(account):
    """
    Convert an account dictionary to a unique string representation.
    
    Args:
        account (dict): Account dictionary with type, owner, country, institution, 
                       plaid_account_type, and name fields
    
    Returns:
        str: Unique string representation of the account
    """
    return f'{account["type"]}:{account["owner"]}:{account["country"]}:{account["institution"]}:{account["plaid_account_type"]}:{account["name"]}'


def create_merged_transaction(from_txn, to_txn, description):
    """
    Create a new merged transaction from two existing transactions.
    
    Args:
        from_txn (dict): The source transaction
        to_txn (dict): The destination transaction
        description (str): Description for the merged transaction
    
    Returns:
        dict: New merged transaction
    """
    payer = from_txn["from_account"]["owner"]
    payee = to_txn["to_account"]["owner"]
    
    return {
        "date": max(from_txn["date"], to_txn["date"]),
        "payee": payee,
        "desc": description,
        "from_account": from_txn["from_account"],
        "to_account": to_txn["to_account"],
        "unit": from_txn["unit"],
        "metadata": {
            "payer": payer,
            "from_id": from_txn["metadata"]["id"],
            "to_id": to_txn["metadata"]["id"]
        },
        "tags": [],
        "amount": from_txn["amount"]
    }


def merge_self_transfers(transactions):
    """
    Merge self-transfers (transactions between accounts owned by the same person).
    
    Args:
        transactions (list): List of beancount transactions
    
    Returns:
        tuple: (merged_transactions, processed_indices)
    """
    merged_transactions = []
    processed_indices = set()
    
    for i, from_txn in enumerate(transactions):
        # Skip if already processed or not from an asset account
        if i in processed_indices or from_txn["from_account"]["type"] != "Assets":
            continue
        
        # Look for matching transaction to merge with
        for j, to_txn in enumerate(transactions):
            # Skip if already processed, same transaction, or doesn't match criteria
            if (j in processed_indices or 
                j == i or 
                from_txn["unit"] != to_txn["unit"] or 
                from_txn["amount"] != to_txn["amount"]):
                continue
            
            # Must be to an asset or liability account
            if to_txn["to_account"]["type"] not in ("Assets", "Liabilities"):
                continue
            
            # Must be same owner (self-transfer)
            if from_txn["from_account"]["owner"] != to_txn["to_account"]["owner"]:
                continue
            
            # Must be different accounts (not same account)
            if account_to_string(from_txn["from_account"]) == account_to_string(to_txn["to_account"]):
                continue
            
            # Create merged transaction
            merged_txn = create_merged_transaction(from_txn, to_txn, "self transfer")
            merged_transactions.append(merged_txn)
            processed_indices.update([i, j])
            break
    
    return merged_transactions, processed_indices


def merge_inter_transfers(transactions, processed_indices):
    """
    Merge inter-transfers (transactions between different people on the same date).
    
    Args:
        transactions (list): List of beancount transactions
        processed_indices (set): Set of already processed transaction indices
    
    Returns:
        tuple: (merged_transactions, updated_processed_indices)
    """
    merged_transactions = []
    updated_processed_indices = processed_indices.copy()
    
    for i, from_txn in enumerate(transactions):
        # Skip if already processed or not from an asset account
        if i in updated_processed_indices or from_txn["from_account"]["type"] != "Assets":
            continue
        
        # Look for matching transaction to merge with
        for j, to_txn in enumerate(transactions):
            # Skip if already processed, same transaction, or doesn't match criteria
            if (j in updated_processed_indices or 
                j == i or 
                from_txn["unit"] != to_txn["unit"] or 
                to_txn["to_account"]["type"] != "Assets" or 
                from_txn["amount"] != to_txn["amount"] or 
                to_txn["to_account"]["owner"] == from_txn["from_account"]["owner"]):
                continue
            
            # Must be on the same date
            if from_txn["date"] != to_txn["date"]:
                continue
            
            # Create merged transaction
            payer = from_txn["from_account"]["owner"]
            payee = to_txn["to_account"]["owner"]
            description = f"transfer {payer} -> {payee}"
            
            merged_txn = create_merged_transaction(from_txn, to_txn, description)
            merged_transactions.append(merged_txn)
            updated_processed_indices.update([i, j])
            break
    
    return merged_transactions, updated_processed_indices


def add_unprocessed_transactions(transactions, processed_indices, merged_transactions):
    """
    Add transactions that weren't merged to the final result.
    
    Args:
        transactions (list): Original list of transactions
        processed_indices (set): Set of processed transaction indices
        merged_transactions (list): List of already merged transactions
    
    Returns:
        list: Complete list of transactions (merged + unprocessed)
    """
    final_transactions = merged_transactions.copy()
    
    for i, txn in enumerate(transactions):
        if i not in processed_indices:
            final_transactions.append(txn)
    
    return final_transactions


def main():
    """
    Main function to process and merge beancount transactions.
    """
    # Load transactions from file
    _, bc_txns = util.load()
    
    print(f"Processing {len(bc_txns)} transactions...")
    
    # Step 1: Merge self-transfers (same owner, different accounts)
    print("Merging self-transfers...")
    self_transfers, processed_indices = merge_self_transfers(bc_txns)
    print(f"Found {len(self_transfers)} self-transfer pairs")
    
    # Step 2: Merge inter-transfers (different owners, same date)
    print("Merging inter-transfers...")
    inter_transfers, final_processed_indices = merge_inter_transfers(bc_txns, processed_indices)
    print(f"Found {len(inter_transfers)} inter-transfer pairs")
    
    # Step 3: Add unprocessed transactions
    all_merged = self_transfers + inter_transfers
    final_transactions = add_unprocessed_transactions(bc_txns, final_processed_indices, all_merged)
    
    print(f"Original transactions: {len(bc_txns)}")
    print(f"Merged transactions: {len(all_merged)}")
    print(f"Final transactions: {len(final_transactions)}")
    
    # Save results
    util.dump(final_transactions)
    print("Merged transactions saved successfully!")


if __name__ == "__main__":
    main()
