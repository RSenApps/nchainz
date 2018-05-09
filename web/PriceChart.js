export default class PriceChart {
  constructor(elementId, orderbook) {
    let ctx = document.getElementById(elementId).getContext('2d')

    this.numPrices = 60
    this.prices = []
    this.chartData = []
    this.yMin = Number.MAX_VALUE
    this.yMax = 0

    this.chart = new Chart(ctx, PriceChart.initChartOptions(orderbook))
    this.updateChart(orderbook)
  }

  updateChart(orderbook) {
    this.setChartData(orderbook)

    this.chart.data.datasets[0].data = this.chartData
    this.chart.options.scales.xAxes[0].ticks.min = this.prices.length - this.numPrices
    this.chart.options.scales.xAxes[0].ticks.max = this.prices.length - 1
    this.chart.options.scales.yAxes[0].ticks.suggestedMin = this.yMin - orderbook.marketPrice / 200
    this.chart.options.scales.yAxes[0].ticks.suggestedMax = this.yMax + orderbook.marketPrice / 200

    this.chart.update(300)
  }

  setChartData(orderbook) {
    let price = orderbook.marketPrice
    this.prices.push(price)

    if (price < this.yMin) {
      this.yMin = price
    }
    if (price > this.yMax) {
      this.yMax = price
    }

    this.chartData.push({
      x: this.prices.length-1,
      y: price,
    })
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
            label: "Price",
            borderWidth: 1,
            borderColor: 'rgb(50,100,200)',
            backgroundColor: 'rgba(50,100,200,0.2)',
            pointRadius: 0,
            lineTension: 0,
          },
        ],
      },

      options: {
        responsive: false,
        scales: {
          yAxes: [{
            scaleLabel: {
              display: false,
              labelString: 'Depth (' + orderbook.quoteSymbol + ')',
              fontColor: '#444',
            },
          }],
          xAxes: [{
            display: false,
            type: 'linear',
            position: 'bottom',
            scaleLabel: {
              display: false,
              labelString: 'Price (' + orderbook.baseSymbol + ')',
              fontColor: '#444',
            },
          }],
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
