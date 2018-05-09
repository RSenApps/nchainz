const REFRESH_INTERVAL = 1000
let book = "ETH/USD"

getBook(initChart)
setInterval(() => getBook(updateChart), REFRESH_INTERVAL)

function getBook(callback) {
  var req = new XMLHttpRequest()
  
  req.onreadystatechange = () => {
    if (req.readyState == XMLHttpRequest.DONE) {
      if (req.status == 200) {
        orderbook = parseInput(req.responseText)
        console.log(orderbook)
        callback(orderbook)

      } else {
        console.log(req.status)
      }
    }
  }

  req.open('GET', '/book/' + book)
  req.send()
}

function parseInput(serialized) {
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

  function getChartData(orders) {
    var chartData = []
    var cumAmt = 0
    var lastPrice = -1

    for (let order of orders) {
      cumAmt += order.quoteAmt

      if (order.price == lastPrice) {
        chartData[chartData.length-1].y = cumAmt
      } else {
        chartData.push({x: order.price, y: cumAmt})
      }

      lastPrice = order.price
    }

    return chartData
  }

  var chunks = serialized.split("\n")
  var marketPrice = parseFloat(chunks[0])
  var quoteSymbol = chunks[1]
  var baseSymbol = chunks[2]
  var quoteOrders = chunks[3].split(",").map(parseOrder).sort((a,b) => b.price - a.price)
  var baseOrders = chunks[4].split(",").map(parseOrder).sort((a,b) => a.price - b.price)

  var quoteData = getChartData(quoteOrders)
  var baseData = getChartData(baseOrders)

  var minPrice = quoteOrders[quoteOrders.length-1].price
  var maxPrice = baseOrders[baseOrders.length-1].price
  var xMin, xMax
  
  if (marketPrice - minPrice > maxPrice - marketPrice) {
    let buffer = 0.2 * (marketPrice - minPrice)
    xMin = minPrice - buffer
    xMax = marketPrice + (marketPrice - minPrice) + buffer
  } else {
    let buffer = 0.2 * (maxPrice - marketPrice)
    xMin = marketPrice - (maxPrice - marketPrice) - buffer
    xMax = maxPrice + buffer
  }

  quoteData.push({x: xMin, y: quoteData[quoteData.length-1].y})
  baseData.push({x: xMax, y: baseData[baseData.length-1].y})

  return {
    marketPrice,
    quoteSymbol,
    quoteOrders,
    quoteData,
    baseSymbol,
    baseOrders,
    baseData,
    xMin,
    xMax,
  }
}

function initChart(orderbook) {
  var ctx = document.getElementById('orderbook').getContext('2d');
  chart = new Chart(ctx, {
    type: 'line',

    data: {
      datasets: [
        {
          label: "Bids",
          borderWidth: 1,
          borderColor: 'rgb(131,175,44)',
          backgroundColor: 'rgb(150,200,50)',
          steppedLine: true,
          pointRadius: 0,
        },
        {
          label: "Asks",
          borderWidth: 1,
          borderColor: 'rgb(175,87,44)',
          backgroundColor: 'rgb(200,100,50)',
          steppedLine: true,
          pointRadius: 0,
        },
      ],
    },

    options: {
      responsive: false,
      scales: {
        yAxes: [{
          ticks: {
            beginAtZero: true,
          },
          scaleLabel: {
            display: false,
            labelString: 'Depth (' + orderbook.quoteSymbol + ')',
            fontColor: '#444',
          },
        }],
        xAxes: [{
          type: 'linear',
          position: 'bottom',
          scaleLabel: {
            display: false,
            labelString: 'Price (' + orderbook.baseSymbol + ')',
            fontColor: '#444',
          },
        }],
      },
      title: {
        display: true,
        position: 'top',
        fontColor: '#444',
        fontSize: 16,
      },
      legend: {
        display: false,
      },
      animation: {
        duration: 300,
      },
    },
  })

  updateChart(orderbook)
}

function updateChart(orderbook) {
  chart.data.datasets[0].data = orderbook.quoteData
  chart.data.datasets[1].data = orderbook.baseData
  chart.options.scales.xAxes[0].ticks.min = orderbook.xMin
  chart.options.scales.xAxes[0].ticks.max = orderbook.xMax
  chart.options.title.text = orderbook.quoteSymbol + '/' + orderbook.baseSymbol + ": " + orderbook.marketPrice

  chart.update(300)
}

/*
var serialized = "707\nETH\nUSD\n"
  + "706 2 1,705 10 1,703.5 10 1,703 50 1,701 30 1,700 10 1,699 20 1\n"
  + "708 3 1,708.5 5 1,709.25 10 1,710 30 1,711 5 1,712 15 1,713 10 1,714 25 1"
*/
