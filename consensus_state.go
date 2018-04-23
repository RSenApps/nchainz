package main

type ConsensusStateToken struct {
	openOrders []Order
	balances map[string]uint64
}

type ConsensusState struct {
	tokenStates []ConsensusStateToken
	unconfirmedMatchIDs map[uint64]bool
	createdTokens []TokenInfo
}

func (state *ConsensusState) AddOrder(token int, order Order) {

}

func (state *ConsensusState) RollbackOrder(token int, order Order) {

}

func (state *ConsensusState) AddCancelOrder(token int, cancelOrder CancelOrder) {

}

func (state *ConsensusState) RollbackCancelOrder(token int, cancelOrder CancelOrder) {

}

func (state *ConsensusState) AddTransactionConfirmed(token int, transactionConfirmed TransactionConfirmed) {

}

func (state *ConsensusState) RollbackTransactionConfirmed(token int, transactionConfirmed TransactionConfirmed) {

}

func (state *ConsensusState) AddTransfer(token int, transfer Transfer) {

}

func (state *ConsensusState) RollbackTransfer(token int, transfer Transfer) {

}

func (state *ConsensusState) AddMatch(match Match) {

}

func (state *ConsensusState) RollbackMatch(match Match) {

}

func (state *ConsensusState) AddCancelMatch(cancelMatch CancelMatch) {

}

func (state *ConsensusState) RollbackCancelMatch(cancelMatch CancelMatch) {

}

func (state *ConsensusState) AddCreateToken(createToken CreateToken) {

}

func (state *ConsensusState) RollbackCreateToken(createToken CreateToken) {

}
