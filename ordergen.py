import random
import os
import time

while True:
    price = int(random.normalvariate(1000, 100))
    
    if random.randint(0,1) == 0:
        print "nchainz order 1 ETH " + str(price) + " USD sam"
        os.system("nchainz order 1 ETH " + str(price) + " USD sam")
    else:
        os.system("nchainz order " + str(price) + " USD 1 ETH vit")

    time.sleep(1)
