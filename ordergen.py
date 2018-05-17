import random
import math
import os
import time

i = 0
while True:
    i += 1
    price = int(random.normalvariate(1000, 100))
    ethAmt = 1
    usdAmt = price * ethAmt
    
    if random.randint(0,1) == 0:
        command = "nchainz order " + str(ethAmt) + " ETH " + str(usdAmt) + " USD 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K"
    else:
        command = "nchainz order " + str(usdAmt) + " USD " + str(ethAmt) +" ETH 1CufpgmhVmV7fujYHqFCqUdJe5vwhcc96K"

    print command
    os.system(command)

    delay = -5 * math.cos(i*3.14159/15) + 5
    time.sleep(4)
