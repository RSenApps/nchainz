export default class DataLoader {
  constructor(bookname) {
    this.bookname = bookname
  }

  getChains(callback) {
    var req = new XMLHttpRequest()

    req.onreadystatechange = () => {
      if (req.readyState == XMLHttpRequest.DONE) {
        if (req.status == 200) {
          let chains = DataLoader.parseChainsInput(req.responseText)
          console.log(chains)
          callback(chains)

        } else {
          console.log(req.status)
        }
      }
    }

    req.open('GET', '/chains')
    req.send()
  }

  getBook(callback) {
    var req = new XMLHttpRequest()

    req.onreadystatechange = () => {
      if (req.readyState == XMLHttpRequest.DONE) {
        if (req.status == 200) {
          let orderbook = DataLoader.parseBookInput(req.responseText)
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

  static parseChainsInput(serialized) {
    function parseBlock(block) {
      let chunks = block.split(" ")
      let hash = chunks[0]
      let data = [chunks[1], chunks[2], chunks[3]].map(parseInt)

      return {
        hash,
        data
      }
    }

    let chains = {}
    let lines = serialized.split("\n")
    let numChains = parseInt(lines[0])

    let lineNum = 1
    for (let i = 0; i < numChains; i++) {
      let header = lines[lineNum].split(" ")
      lineNum++

      let symbol = header[0]
      let height = parseInt(header[1])
      let amt = parseInt(header[2])

      let blocks = lines.slice(lineNum, lineNum + amt).map(parseBlock)
      blocks[0].height = height

      for (let j = 1; j < blocks.length; j++) {
        blocks[j].height = blocks[j-1].height - 1
      }

      chains[symbol] = blocks
      lineNum += amt
    }

    return chains
  }

  static parseBookInput(serialized) {
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
