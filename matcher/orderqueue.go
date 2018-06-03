package matcher

import (
	"bytes"
	"container/heap"
	"errors"
	"fmt"
)

type OrderQueue struct {
	Items    []*OrderQueueItem
	Side     OrderSide
	IdToItem map[uint64]*OrderQueueItem
}

type OrderQueueItem struct {
	price float64
	order *Order
	index int
}

type OrderSide bool

const (
	BASE  OrderSide = true
	QUOTE OrderSide = false
)

func NewOrderQueue(side OrderSide) *OrderQueue {
	items := make([]*OrderQueueItem, 0)
	idToItem := make(map[uint64]*OrderQueueItem)
	return &OrderQueue{items, side, idToItem}
}

func (oq *OrderQueue) Enq(order *Order) {
	item := &OrderQueueItem{}
	item.order = order
	oq.setItemPrice(item)

	heap.Push(oq, item)
}

func (oq *OrderQueue) Deq() (order *Order, price float64, err error) {
	if oq.Len() == 0 {
		return nil, 0.0, errors.New("empty queue")
	}

	item := heap.Pop(oq).(*OrderQueueItem)
	return item.order, item.price, nil
}

func (oq *OrderQueue) Peek() (order *Order, price float64, err error) {
	if oq.Len() == 0 {
		return nil, 0.0, errors.New("empty queue")
	}

	item := oq.Items[0]
	return item.order, item.price, nil
}

func (oq *OrderQueue) Remove(id uint64) error {
	item, exists := oq.IdToItem[id]
	if !exists {
		LogRed("Attempte to remove nonexistent order %v", id)
		return errors.New("order does not exist in queue")
	}

	heap.Remove(oq, item.index)
	return nil
}

func (oq *OrderQueue) GetOrder(id uint64) (*Order, bool) {
	item, exists := oq.IdToItem[id]

	if !exists {
		return nil, false
	}

	return item.order, true
}

func (oq *OrderQueue) FixPrice(id uint64) {
	item := oq.IdToItem[id]
	oq.setItemPrice(item)
	heap.Fix(oq, item.index)
}

func (oq *OrderQueue) String() string {
	var buffer bytes.Buffer

	for _, oqi := range oq.Items {
		buffer.WriteString(oq.oqiString(oqi))
	}

	if oq.Side == BASE {
		return fmt.Sprintf("BASE %s", buffer.String())
	} else {
		return fmt.Sprintf("QUOTE %s", buffer.String())
	}
}

func (oq *OrderQueue) oqiString(oqi *OrderQueueItem) string {
	if oq.Side == BASE {
		return fmt.Sprintf("<%v @ %f #%v>", oqi.order.AmountToSell, oqi.price, oqi.order.ID)
	} else {
		return fmt.Sprintf("<%v @ %f #%v>", oqi.order.AmountToBuy, oqi.price, oqi.order.ID)
	}
}

func (oq *OrderQueue) Serial() string {
	var buffer bytes.Buffer

	for i, oqi := range oq.Items {
		if i != 0 {
			buffer.WriteString(",")
		}

		buffer.WriteString(oq.oqiSerial(oqi))
	}

	return buffer.String()
}

func (oq *OrderQueue) oqiSerial(oqi *OrderQueueItem) string {
	if oq.Side == BASE {
		return fmt.Sprintf("%f %v %v", oqi.price, oqi.order.AmountToSell, oqi.order.AmountToBuy)
	} else {
		return fmt.Sprintf("%f %v %v", oqi.price, oqi.order.AmountToBuy, oqi.order.AmountToSell)
	}

}

func (oq *OrderQueue) setItemPrice(item *OrderQueueItem) {
	if oq.Side == QUOTE {
		item.price = float64(item.order.AmountToSell) / float64(item.order.AmountToBuy)
	} else { // oq.Side == BASE
		item.price = float64(item.order.AmountToBuy) / float64(item.order.AmountToSell)
	}
}

/////////////////////
// Utils used by heap

func (oq *OrderQueue) Len() int {
	return len(oq.Items)
}

func (oq *OrderQueue) Less(i, j int) bool {
	if oq.Side == BASE {
		return oq.Items[i].price < oq.Items[j].price
	} else { // oq.Side == QUOTE
		return oq.Items[i].price > oq.Items[j].price
	}
}

func (oq *OrderQueue) Swap(i, j int) {
	oq.Items[i], oq.Items[j] = oq.Items[j], oq.Items[i]
	oq.Items[i].index = i
	oq.Items[j].index = j
}

func (oq *OrderQueue) Push(x interface{}) {
	item := x.(*OrderQueueItem)

	if old, exists := oq.IdToItem[item.order.ID]; exists {
		msg := fmt.Sprintf("Duplicate order id %v, old %v, new %v\n%v", item.order.ID, oq.oqiString(old), oq.oqiString(item), oq)
		LogRed(msg)
		//panic(msg)
		return
	}

	item.index = len(oq.Items)
	oq.Items = append(oq.Items, item)
	oq.IdToItem[item.order.ID] = item
}

func (oq *OrderQueue) Pop() interface{} {
	item := oq.Items[len(oq.Items)-1]
	item.index = -1
	oq.Items = oq.Items[0 : len(oq.Items)-1]
	delete(oq.IdToItem, item.order.ID)
	return item
}
