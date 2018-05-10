export default class DepthChart {
  constructor(elementId, orderbook) {
    let ctx = document.getElementById(elementId).getContext('2d')
    this.chart = new Chart(ctx, DepthChart.initChartOptions(orderbook))
    this.updateChart(orderbook)
  }

  updateChart(orderbook) {
    let chartData = DepthChart.getChartData(orderbook)

    this.chart.data.datasets[0].data = chartData.quoteData
    this.chart.data.datasets[1].data = chartData.baseData
    this.chart.options.scales.xAxes[0].ticks.min = chartData.xMin
    this.chart.options.scales.xAxes[0].ticks.max = chartData.xMax
    this.chart.options.title.text = orderbook.quoteSymbol + '/' + orderbook.baseSymbol + ": " + orderbook.marketPrice

    this.chart.update(300)
  }

  static getChartData(orderbook) {
    let quoteData = DepthChart.cumulateOrders(orderbook.quoteOrders)
    let baseData = DepthChart.cumulateOrders(orderbook.baseOrders)

    let minPrice = orderbook.quoteOrders[orderbook.quoteOrders.length-1].price
    let maxPrice = orderbook.baseOrders[orderbook.baseOrders.length-1].price
    let xMin, xMax

    if (orderbook.marketPrice - minPrice > maxPrice - orderbook.marketPrice) {
      let buffer = 0.2 * (orderbook.marketPrice - minPrice)
      xMin = minPrice - buffer
      xMax = orderbook.marketPrice + (orderbook.marketPrice - minPrice) + buffer
    } else {
      let buffer = 0.2 * (maxPrice - orderbook.marketPrice)
      xMin = orderbook.marketPrice - (maxPrice - orderbook.marketPrice) - buffer
      xMax = maxPrice + buffer
    }

    quoteData.push({x: xMin, y: quoteData[quoteData.length-1].y})
    baseData.push({x: xMax, y: baseData[baseData.length-1].y})

    return {
      quoteData,
      baseData,
      xMin,
      xMax,
    }
  }

  static cumulateOrders(orders) {
    var data = []
    var cumAmt = 0
    var lastPrice = -1

    for (let order of orders) {
      cumAmt += order.quoteAmt

      if (order.price == lastPrice) {
        data[data.length-1].y = cumAmt
      } else {
        data.push({x: order.price, y: cumAmt})
      }

      lastPrice = order.price
    }

    return data
  }

  static initChartOptions(orderbook) {
    let removeFirstLastLabel = (scaleInstance) => {
      scaleInstance.ticks[0] = null
      scaleInstance.ticks[scaleInstance.ticks.length - 1] = null
      scaleInstance.ticksAsNumbers[0] = null
      scaleInstance.ticksAsNumbers[scaleInstance.ticksAsNumbers.length - 1] = null
    }

    return {
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
            afterTickToLabelConversion: removeFirstLastLabel,
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
    }
  }
}
