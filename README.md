# N Chainz

Centralized exchanges rely on trusting that their owners will take the proper security precautions. This has led to many incidents of stolen cryptocurrency adding up to billions of dollars worth of losses, and is a stark contrast to the decentralization of the rest of the space. On the other hand, decentralized exchanges have the potential to be much more secure: theoretically, users are not vulnerable to server downtime and hacks, and can retain anonymity. 

We present **N Chainz**, a decentralized cryptocurrency exchange with a unique multi-chain architecture. We built N Chainz from the ground up, and included features such as block generation, limit orders, and the ability to trade a base token with another token. More details are included in our [whitepaper](http://github.com/RSenApps/nchainz/blob/master/proj-final.pdf).

## System Overview

![A system overview](images/0.png?raw=true)

## Multiple Blockchains

![A system overview](images/1.png?raw=true)

## Web UI: Order Matching & Price Chart

![A system overview](images/3.png?raw=true)

## Setup
	go get -d github.com/rsenapps/nchainz
	go install github.com/rsenapps/nchainz

## Usage
``nchainz COMMAND [ARGS]``

### Account management

* ``createwallet``  
Create a wallet with a pair of keys  
* ``getbalance ADDRESS SYMBOL``  
Get the balance for an address  
* ``printaddresses``  
Print all adddreses in wallet file  

### Creating transactions

* ``order BUY_AMT BUY_SYMBOL SELL_AMT SELL_SYMBOL ADDRESS``  
Create an ORDER transaction
* ``transfer AMT SYMBOL FROM TO``  
Create a TRANSFER transaction
* ``freeze AMT SYMBOL FROM UNFREEZE_BLOCK``  
Create a FREEZE tokens transaction
* ``cancel SYMBOL ORDER_ID``  
Create a CANCEL_ORDER transaction
* ``claim AMT SYMBOL ADDRESS``  
Create a CLAIM_FUNDS transaction
* ``create SYMBOL SUPPLY DECIMALS ADDRESS``  
Create a CREATE_TOKEN transaction

### Running a node or miner

* ``node HOSTNAME:PORT``  
  Start up a full node providing your hostname on the given port
* ``printchain DB SYMBOL``  
  Prints all the blocks in the blockchain
* ``webserver PORT``  
  Run a webserver on the given port
  
## Authors

* [Ryan Senanayake](http://rsenapps.com/)
* [Nicholas Egan](http://nicholasegan.me/)
* [Elizabeth Wei](http://lizziew.github.io/)

## License
The GNU Affero General Public License (see LICENSE)
