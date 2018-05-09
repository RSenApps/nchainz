import DataLoader from './DataLoader.js'
import DepthChart from './DepthChart.js'

const REFRESH_INTERVAL = 1000

let dl = new DataLoader("ETH/USD")

dl.getBook((orderbook) => {
  let chart = new DepthChart(orderbook)

  setInterval(() => dl.getBook((orderbook) => {
    chart.updateChart(orderbook)
  }), REFRESH_INTERVAL)
})

/*
var serialized = "707\nETH\nUSD\n"
  + "706 2 1,705 10 1,703.5 10 1,703 50 1,701 30 1,700 10 1,699 20 1\n"
  + "708 3 1,708.5 5 1,709.25 10 1,710 30 1,711 5 1,712 15 1,713 10 1,714 25 1"
*/
