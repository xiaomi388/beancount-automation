#!/usr/bin/env python3
import util


_, bc_txns = util.load()

# keyword replace
m = {
    ("GOOGLEWEBPASS", "NationalUtility"): ["Home", "HomeBilling"],
}

for i in range(len(bc_txns)):
    for keywords, category in m.items():
        for keyword in keywords:
            if bc_txns[i]["desc"].find(keyword) != -1:
                bc_txns[i]["to_account"]["category"] = category
                break

util.dump(bc_txns)
