import os
import pickledb
import env

meta = pickledb.load(os.path.join(env.runpath, "metadata.db"), auto_dump=False)
txn = pickledb.load(os.path.join(env.runpath, "transaction.db"), auto_dump=False)
