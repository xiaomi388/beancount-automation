from collections import defaultdict

def merge_transfer(args, _):
    """ merge transfer transactions based on the amount"""
    amount_to_args = defaultdict(list)
    for arg in args:
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

