<template>
  <section class="analytics content relative">
    <h1 class="title is-4">
      {{ $t('analytics.sequenceTitle') }}
    </h1>
    <p class="has-text-grey">
      {{ $t('analytics.sequenceDescription') }}
    </p>
    <hr />

    <div class="columns">
      <div class="column is-3">
        <div class="box">
          <p class="heading">{{ $t('analytics.activeContacts') }}</p>
          <p class="title is-4">{{ $utils.niceNumber(stats.activeContacts) }}</p>
        </div>
      </div>
      <div class="column is-3">
        <div class="box">
          <p class="heading">{{ $t('analytics.stepCompletions') }}</p>
          <p class="title is-4">{{ $utils.niceNumber(stats.stepCompletions) }}</p>
        </div>
      </div>
      <div class="column is-3">
        <div class="box">
          <p class="heading">{{ $t('analytics.replyRate') }}</p>
          <p class="title is-4">{{ stats.replyRate }}%</p>
        </div>
      </div>
      <div class="column is-3">
        <div class="box">
          <p class="heading">{{ $t('analytics.conversionRate') }}</p>
          <p class="title is-4">{{ stats.conversionRate }}%</p>
        </div>
      </div>
    </div>

    <section class="charts mt-5">
      <div class="box">
        <h4 class="subtitle is-5">{{ $t('analytics.stepConversionFunnel') }}</h4>
        <chart type="bar" v-if="!loading" :data="chartData" />
      </div>
    </section>
  </section>
</template>

<script>
import Vue from 'vue';
import Chart from '../components/Chart.vue';

export default Vue.extend({
  name: 'SequenceAnalytics',
  components: {
    Chart,
  },
  data() {
    return {
      loading: false,
      stats: {
        activeContacts: 3420,
        stepCompletions: 8910,
        replyRate: 14.2,
        conversionRate: 8.7,
      },
      chartData: {
        labels: ['Step 1: Initial Touch', 'Step 2: Follow-up', 'Step 3: Value Add', 'Step 4: Final Call'],
        datasets: [
          {
            name: 'Contacts Reached',
            values: [3420, 2810, 1950, 1120],
          },
          {
            name: 'Replies',
            values: [240, 210, 180, 110],
          },
        ],
      },
    };
  },
});
</script>
