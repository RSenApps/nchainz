export default class PriceChart {
  constructor(elementId, orderbook) {
    let ctx = document.getElementById(elementId).getContext('2d')
    this.numPrices = 60
    this.prices = []
    this.chart = new Chart(ctx, PriceChart.initChartOptions(orderbook))
    this.updateChart(orderbook)
  }

  updateChart(orderbook) {
    this.prices.push(orderbook.marketPrice)
    let chartData = this.getChartData(orderbook)

    this.chart.data.datasets[0].data = chartData.data
    this.chart.options.scales.xAxes[0].ticks.min = this.prices.length - this.numPrices
    this.chart.options.scales.xAxes[0].ticks.max = this.prices.length - 1
    this.chart.options.scales.yAxes[0].ticks.suggestedMin = chartData.yMin
    this.chart.options.scales.yAxes[0].ticks.suggestedMax = chartData.yMax

    this.chart.update(300)
  }

  getChartData(orderbook) {
    let data = []
    
    let yMin = Number.MAX_VALUE
    let yMax = 0

    for (let i = 0; i < this.prices.length; i++) {
      let price = this.prices[i]
      if (price < yMin) {
        yMin = price
      }
      if (price > yMax) {
        yMax = price
      }

      data.push({
        x: i,
        y: price,
      })
    }

    yMin -= orderbook.marketPrice / 200
    yMax += orderbook.marketPrice / 200
    console.log(yMin, yMax)

    return {
      data,
      yMin,
      yMax,
    }
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
