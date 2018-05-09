export default class DataLoader {
  constructor(bookname) {
    this.bookname = bookname
  }

  getBook(callback) {
    var req = new XMLHttpRequest()
    
    req.onreadystatechange = () => {
      if (req.readyState == XMLHttpRequest.DONE) {
        if (req.status == 200) {
          let orderbook = DataLoader.parseInput(req.responseText)
          console.log(orderbook)
          callback(orderbook)

        } else {
          console.log(req.status)
        }
      }
    }

    req.open('GET', '/book/' + this.bookname)
    req.send()
  }

  static parseInput(serialized) {
    function parseOrder(order) {
      var chunks = order.split(" ")
      var price = parseFloat(chunks[0])
      var quoteAmt = parseInt(chunks[1])
      var baseAmt = parseInt(chunks[2])

      return {
        price,
        quoteAmt,
        baseAmt,
      }
    }

    var chunks = serialized.split("\n")
    var marketPrice = parseFloat(chunks[0])
    var quoteSymbol = chunks[1]
    var baseSymbol = chunks[2]
    var quoteOrders = chunks[3].split(",").map(parseOrder).sort((a,b) => b.price - a.price)
    var baseOrders = chunks[4].split(",").map(parseOrder).sort((a,b) => a.price - b.price)

    return {
      marketPrice,
      quoteSymbol,
      quoteOrders,
      baseSymbol,
      baseOrders,
    }
  }
}
