<template>
  <section class="analytics content relative">
    <h1 class="title is-4">
      {{ $t('analytics.templateTitle') }}
    </h1>
    <p class="has-text-grey">
      {{ $t('analytics.templateDescription') }}
    </p>
    <hr />

    <div class="columns">
      <div class="column is-3">
        <div class="box">
          <p class="heading">{{ $t('analytics.totalExecutions') }}</p>
          <p class="title is-4">{{ $utils.niceNumber(stats.totalExecutions) }}</p>
        </div>
      </div>
      <div class="column is-3">
        <div class="box">
          <p class="heading">{{ $t('analytics.avgLatency') }}</p>
          <p class="title is-4">{{ stats.avgLatencyMs }} ms</p>
        </div>
      </div>
      <div class="column is-3">
        <div class="box">
          <p class="heading">{{ $t('analytics.totalTokens') }}</p>
          <p class="title is-4">{{ $utils.niceNumber(stats.totalTokens) }}</p>
        </div>
      </div>
      <div class="column is-3">
        <div class="box">
          <p class="heading">{{ $t('analytics.errorRate') }}</p>
          <p class="title is-4">{{ stats.errorRate }}%</p>
        </div>
      </div>
    </div>

    <section class="charts mt-5">
      <div class="box">
        <h4 class="subtitle is-5">{{ $t('analytics.jitExecutionMetrics') }}</h4>
        <chart type="bar" v-if="!loading" :data="chartData" />
      </div>
    </section>
  </section>
</template>

<script>
import Vue from 'vue';
import Chart from '../components/Chart.vue';

export default Vue.extend({
  name: 'TemplateAnalytics',
  components: {
    Chart,
  },
  data() {
    return {
      loading: false,
      stats: {
        totalExecutions: 12480,
        avgLatencyMs: 342,
        totalTokens: 1845000,
        errorRate: 0.12,
      },
      chartData: {
        labels: ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'],
        datasets: [
          {
            name: 'JIT Completions',
            values: [1200, 1900, 2400, 2100, 2800, 1100, 980],
          },
        ],
      },
    };
  },
});
</script>
