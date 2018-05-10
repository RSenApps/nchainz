import DataLoader from './DataLoader.js'
import DepthChart from './DepthChart.js'
import PriceChart from './PriceChart.js'

const REFRESH_INTERVAL = 3000
const DEFAULT_BOOK = "ETH/USD"

let bookname
let url = new URL(window.location)
let chunks = url.hash.split("/")

if (chunks.length != 3 || chunks[0] != '#') {
  window.location = '/#/' + DEFAULT_BOOK
  bookname = DEFAULT_BOOK
} else {
  bookname = chunks[1] + '/' + chunks[2]
}

let dl = new DataLoader(bookname)

dl.getBook((orderbook) => {
  let depthChart = new DepthChart('depthChart', orderbook)
  let priceChart = new PriceChart('priceChart', orderbook)

  setInterval(() => dl.getBook((orderbook) => {
    depthChart.updateChart(orderbook)
    priceChart.updateChart(orderbook)
  }), REFRESH_INTERVAL)
})

dl.getChains((chains) => {
  /*setInterval(() => dl.getBook((orderbook) => {
    depthChart.updateChart(orderbook)
    priceChart.updateChart(orderbook)
  }), REFRESH_INTERVAL)
  */
})
