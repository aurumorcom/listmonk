<template>
  <section class="analytics content relative">
    <h1 class="title is-4">
      {{ $t('analytics.title') }}
    </h1>
    <div v-if="serverConfig.privacy.disable_tracking || !serverConfig.privacy.individual_tracking"
      class="notification is-info">
      <template v-if="serverConfig.privacy.disable_tracking">
        {{ $t('analytics.trackingDisabled') }}
      </template>
      <template v-else-if="!serverConfig.privacy.individual_tracking">
        {{ $t('analytics.nonIndividualTracking') }}
      </template>
    </div>
    <hr />

    <form @submit.prevent="onSubmit">
      <div class="columns">
        <div class="column is-6">
          <b-field label="Sequences" label-position="on-border">
            <b-taginput v-model="form.items" :data="queriedItems" name="items" ellipsis icon="tag-outline"
              placeholder="Select sequences or steps..." autocomplete :allow-new="false" :open-on-focus="true"
              :before-adding="isItemSelected" @typing="queryItems" @focus="queryItems" field="name"
              :loading="isSearchLoading" />
          </b-field>
        </div>

        <div class="column is-5">
          <div class="columns">
            <div class="column is-6">
              <b-field data-cy="from" :label="$t('analytics.fromDate')" label-position="on-border">
                <b-datetimepicker v-model="form.from" icon="calendar-clock" :timepicker="{ hourFormat: '24' }"
                  :datetime-formatter="formatDateTime" @input="onFromDateChange" />
              </b-field>
            </div>
            <div class="column is-6">
              <b-field data-cy="to" :label="$t('analytics.toDate')" label-position="on-border">
                <b-datetimepicker v-model="form.to" icon="calendar-clock" :timepicker="{ hourFormat: '24' }"
                  :datetime-formatter="formatDateTime" @input="onToDateChange" />
              </b-field>
            </div>
          </div>
        </div>

        <div class="column is-1">
          <b-button native-type="submit" type="is-primary" icon-left="magnify" :disabled="form.items.length === 0"
            data-cy="btn-search" />
        </div>
      </div>
    </form>

    <section class="charts mt-5">
      <div class="chart" v-for="(v, k) in charts" :key="k">
        <div class="columns">
          <div class="column is-9">
            <b-loading v-if="v.loading" :active="v.loading" :is-full-page="false" />
            <h4>
              {{ v.name }}
              <span v-if="v.type !== 'bar'" class="has-text-grey-light">({{ $utils.niceNumber(counts[k]) }})</span>
            </h4>
            <chart :type="v.type" v-if="!v.loading" :data="v.data" :on-click="v.onClick" />
          </div>
          <div class="column is-2 donut-container">
            <chart type="donut" v-if="!v.loading" :data="v.donutData" />
          </div>
        </div>
      </div>
    </section>
  </section>
</template>

<script>
import dayjs from 'dayjs';
import Vue from 'vue';
import { mapState } from 'vuex';
import Chart from '../components/Chart.vue';
import { colors } from '../constants';

const chartColorRed = '#ee7d5b';
const chartColors = [
  colors.primary,
  '#FFB50D',
  '#41AC9C',
  chartColorRed,
  '#7FC7BC',
  '#3a82d6',
  '#688ED9',
  '#FFC43D',
];

export default Vue.extend({
  name: 'SequenceAnalytics',
  components: {
    Chart,
  },

  data() {
    return {
      isSearchLoading: false,
      queriedItems: [],

      counts: {
        views: 0,
        clicks: 0,
        bounces: 0,
        links: 0,
      },
      urls: [],
      charts: {
        views: {
          name: this.$t('campaigns.views'),
          type: 'line',
          data: null,
          fn: this.$api.getCampaignViewCounts,
          chartFn: this.makeCharts,
          loading: false,
        },

        clicks: {
          name: this.$t('campaigns.clicks'),
          type: 'line',
          data: null,
          fn: this.$api.getCampaignClickCounts,
          chartFn: this.makeCharts,
          loading: false,
        },

        bounces: {
          name: this.$t('globals.terms.bounces'),
          type: 'line',
          data: null,
          fn: this.$api.getCampaignBounceCounts,
          chartFn: this.makeCharts,
          donutColor: chartColorRed,
          loading: false,
        },

        links: {
          name: this.$t('analytics.links'),
          type: 'bar',
          data: null,
          loading: false,
          fn: this.$api.getCampaignLinkCounts,
          chartFn: this.makeLinksChart,
          onClick: this.onLinkClick,
        },
      },

      form: {
        items: [],
        from: null,
        to: null,
      },
    };
  },

  methods: {
    onFromDateChange() {
      if (this.form.from > this.form.to) {
        this.form.to = dayjs(this.form.from).add(7, 'day').toDate();
      }
    },

    onToDateChange() {
      if (this.form.from > this.form.to) {
        this.form.from = dayjs(this.form.to).add(-7, 'day').toDate();
      }
    },

    formatDateTime(s) {
      return dayjs(s).format('YYYY-MM-DD HH:mm');
    },

    isItemSelected(item) {
      return !this.form.items.find((i) => i.id === item.id && i.isStep === item.isStep);
    },

    makeLinksChart(typ, items, data) {
      const labels = data.map((l) => {
        try {
          this.urls.push(l.url);
          const u = new URL(l.url);
          if (l.url.length > 80) {
            return `${u.hostname}${u.pathname.substr(0, 50)}..`;
          }
          return u.hostname + u.pathname;
        } catch {
          return l.url;
        }
      });

      const out = {
        labels,
        datasets: [
          {
            data: data.map((l) => l.count),
            backgroundColor: chartColors,
          }],
      };

      return { points: out, donut: null };
    },

    makeCharts(typ, items, data) {
      const itemMap = items.reduce((obj, i) => {
        const out = { ...obj };
        const key = `${i.isStep ? 'step' : 'seq'}_${i.id}`;
        out[key] = i;
        return out;
      }, {});
      const keys = Object.keys(itemMap);

      const lines = keys.map((k, n) => {
        const targetItem = itemMap[k];
        const points = data.filter((item) => (item.campaignId === targetItem.id || item.sequenceId === targetItem.id || item.stepId === targetItem.id));

        return {
          label: targetItem.name,
          data: points.map((item) => ({ x: this.formatDateTime(item.timestamp), y: item.count })),
          borderColor: chartColors[n % chartColors.length],
          borderWidth: 2,
          pointHoverBorderWidth: 5,
          pointBorderWidth: 0.5,
        };
      });

      const labels = [];
      const points = keys.map((k) => {
        const targetItem = itemMap[k];
        labels.push(targetItem.name);
        const sum = data.reduce((a, item) => (item.campaignId === targetItem.id || item.sequenceId === targetItem.id || item.stepId === targetItem.id ? a + item.count : a), 0);
        return sum;
      });

      const donut = {
        labels,
        datasets: [{
          data: points, backgroundColor: chartColors, borderWidth: 6,
        }],
      };
      return { points: { datasets: lines }, donut };
    },

    onSubmit() {
      const seqIds = this.form.items.filter((i) => !i.isStep).map((i) => i.id);
      const stepIds = this.form.items.filter((i) => i.isStep).map((i) => i.id);
      this.$router.push({
        query: {
          id: seqIds.length > 0 ? seqIds : undefined,
          step_id: stepIds.length > 0 ? stepIds : undefined,
          from: dayjs(this.form.from).unix(),
          to: dayjs(this.form.to).unix(),
        },
      });
      this.loadAllCharts();
    },

    queryItems(q) {
      this.isSearchLoading = true;
      const selectedSeqs = this.form.items.filter((i) => !i.isStep);

      this.$api.getSequences().then((res) => {
        const seqList = Array.isArray(res) ? res : (res.data || []);
        const matchingSeqs = seqList
          .filter((s) => !q || (s.name && s.name.toLowerCase().includes(q.toLowerCase())))
          .map((s) => ({
            id: s.id,
            name: `#${s.id}: ${s.name}`,
            isStep: false,
          }));

        // Rule: Only show step suggestions AFTER selecting exactly 1 sequence!
        // If 0 sequences are selected or 2+ sequences are selected, show ONLY sequence options.
        if (selectedSeqs.length === 1) {
          const targetSeqId = selectedSeqs[0].id;
          this.$api.getSequenceSteps(targetSeqId).then((stepsRes) => {
            this.isSearchLoading = false;
            const stepList = Array.isArray(stepsRes) ? stepsRes : (stepsRes.data || []);
            const matchingSteps = stepList
              .map((st, idx) => {
                const sNum = st.step_number || (idx + 1);
                const sName = `Step ${sNum}${st.subject ? `: ${st.subject}` : ''} (Seq #${targetSeqId})`;
                return {
                  id: st.id || sNum,
                  sequence_id: targetSeqId,
                  name: sName,
                  isStep: true,
                };
              })
              .filter((st) => !q || st.name.toLowerCase().includes(q.toLowerCase()));

            this.queriedItems = [...matchingSeqs, ...matchingSteps];
          }).catch(() => {
            this.queriedItems = matchingSeqs;
            this.isSearchLoading = false;
          });
        } else {
          this.queriedItems = matchingSeqs;
          this.isSearchLoading = false;
        }
      }).catch(() => {
        this.isSearchLoading = false;
      });
    },

    loadAllCharts() {
      if (this.form.items.length === 0) return;
      Object.keys(this.charts).forEach((k) => {
        this.charts[k].data = null;
        this.charts[k].donutData = null;
        this.getData(k, this.form.items);
      });
    },

    getData(typ, items) {
      if (!items || items.length === 0) return;
      this.charts[typ].loading = true;

      const ids = items.map((i) => i.id);
      this.charts[typ].fn({
        id: ids,
        from: this.form.from,
        to: this.form.to,
      }).then((data) => {
        const list = Array.isArray(data) ? data : (data.data || []);
        this.counts[typ] = list.reduce((sum, d) => sum + (d.count || 0), 0);

        const { points, donut } = this.charts[typ].chartFn(typ, items, list);
        this.charts[typ].data = points;
        this.charts[typ].donutData = donut;
        this.charts[typ].loading = false;
      }).catch(() => {
        this.charts[typ].loading = false;
      });
    },

    onLinkClick(e) {
      const bars = e.chart.getElementsAtEventForMode(e, 'nearest', { intersect: true }, true);
      if (bars.length > 0 && this.urls[bars[0].index]) {
        window.open(this.urls[bars[0].index], '_blank', 'noopener noreferrer');
      }
    },
  },

  computed: {
    ...mapState(['serverConfig']),
  },

  created() {
    const now = dayjs().set('hour', 23).set('minute', 59).set('seconds', 0);
    const weekAgo = now.subtract(7, 'day').set('hour', 0).set('minute', 0);
    const from = this.$route.query.from ? dayjs.unix(this.$route.query.from) : weekAgo;
    const to = this.$route.query.to ? dayjs.unix(this.$route.query.to) : now;
    this.form.from = from.toDate();
    this.form.to = to.toDate();
  },

  mounted() {
    const seqIDs = this.$utils.parseQueryIDs(this.$route.query.id || this.$route.query.sequence_id);
    const stepIDs = this.$utils.parseQueryIDs(this.$route.query.step_id);

    const promises = [];
    seqIDs.forEach((id) => {
      promises.push(this.$api.getSequence(id).then((res) => {
        const seq = res.data || res;
        return { id: seq.id, name: `#${seq.id}: ${seq.name}`, isStep: false };
      }));
    });

    if (seqIDs.length === 1 && stepIDs.length > 0) {
      const seqId = seqIDs[0];
      stepIDs.forEach((stId) => {
        promises.push(Promise.resolve({
          id: stId,
          sequence_id: seqId,
          name: `Step #${stId} (Seq #${seqId})`,
          isStep: true,
        }));
      });
    }

    if (promises.length > 0) {
      this.isSearchLoading = true;
      Promise.allSettled(promises).then((results) => {
        results.forEach((r) => {
          if (r.status === 'fulfilled' && r.value) {
            this.form.items.push(r.value);
          }
        });

        this.$nextTick(() => {
          this.isSearchLoading = false;
          this.loadAllCharts();
        });
      });
    }
  },
});
</script>
