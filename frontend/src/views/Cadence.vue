<template>
  <section class="cadence">
    <header class="columns page-header">
      <div class="column is-10">
        <h1 class="title is-4">
          {{ isNew ? 'New Cadence' : form.name }}
        </h1>
      </div>
      <div class="column has-text-right">
        <b-button type="is-primary" icon-left="content-save" @click="save">
          Save
        </b-button>
      </div>
    </header>

    <div class="box">
      <b-field label="Cadence Name">
        <b-input v-model="form.name" required placeholder="Cold Outreach Sequence" />
      </b-field>

      <b-field label="Status">
        <b-select v-model="form.status">
          <option value="active">Active</option>
          <option value="paused">Paused</option>
          <option value="archived">Archived</option>
        </b-select>
      </b-field>
    </div>

    <div class="box">
      <h2 class="title is-5">Sequence Steps</h2>

      <div v-for="(step, idx) in steps" :key="idx" class="card mb-4 p-4">
        <div class="columns">
          <div class="column is-2">
            <b-field label="Step Number">
              <strong>Step {{ idx + 1 }}</strong>
            </b-field>
          </div>
          <div class="column is-3">
            <b-field label="Condition">
              <b-select v-model="step.condition" expanded>
                <option value="always">Always Send</option>
                <option value="if_read">If Opened / Read</option>
                <option value="if_not_read">If NOT Opened / Read</option>
                <option value="if_clicked">If Link Clicked</option>
              </b-select>
            </b-field>
          </div>
          <div class="column is-3">
            <b-field label="Messenger Target">
              <b-select v-model="step.messenger" expanded>
                <option value="email">Email (SMTP Pool)</option>
                <option value="waha">WhatsApp (WAHA)</option>
              </b-select>
            </b-field>
          </div>
          <div class="column is-2">
            <b-field label="Delay (Days)">
              <b-numberinput v-model="step.delay_days" min="0" />
            </b-field>
          </div>
          <div class="column is-2 has-text-right">
            <b-button type="is-danger" icon-left="trash-can-outline" @click="removeStep(idx)" />
          </div>
        </div>

        <b-field label="Subject">
          <b-input v-model="step.subject" placeholder="Subject line" />
        </b-field>

        <b-field label="Message Body">
          <b-input v-model="step.body" type="textarea" placeholder="Message content..." />
        </b-field>

        <b-field label="Attachment Media IDs (comma separated)">
          <b-input
            :value="step.media_ids ? step.media_ids.join(', ') : ''"
            placeholder="e.g. 101, 102"
            @input="(val) => updateStepMediaIDs(idx, val)"
          />
        </b-field>
      </div>

      <b-button type="is-info" icon-left="plus" @click="addStep">
        Add Step
      </b-button>
    </div>
  </section>
</template>

<script>
export default {
  name: 'Cadence',
  data() {
    return {
      isNew: true,
      form: {
        id: null,
        name: '',
        status: 'active',
      },
      steps: [
        { step_number: 1, delay_days: 0, messenger: 'email', condition: 'always', subject: '', body: '' },
      ],
    };
  },
  mounted() {
    const id = this.$route.params.id;
    if (id && id !== 'new') {
      this.isNew = false;
      this.loadCadence(id);
    }
  },
  methods: {
    loadCadence(id) {
      this.$api.getCadence(id).then((res) => {
        this.form = res.data;
        this.$api.getCadenceSteps(id).then((stepsRes) => {
          this.steps = stepsRes.data.length ? stepsRes.data : this.steps;
        });
      });
    },
    addStep() {
      this.steps.push({
        step_number: this.steps.length + 1,
        delay_days: 2,
        messenger: 'email',
        condition: 'if_not_read',
        subject: '',
        body: '',
        media_ids: [],
      });
    },
    updateStepMediaIDs(idx, val) {
      const ids = val
        .split(',')
        .map((s) => parseInt(s.trim(), 10))
        .filter((n) => !isNaN(n));
      this.$set(this.steps[idx], 'media_ids', ids);
    },
    removeStep(idx) {
      this.steps.splice(idx, 1);
    },
    save() {
      const action = this.isNew
        ? this.$api.createCadence(this.form)
        : this.$api.updateCadence(this.form.id, this.form);

      action.then((res) => {
        const id = res.data.id;
        this.$api.saveCadenceSteps(id, { steps: this.steps }).then(() => {
          this.$router.push({ name: 'cadences' });
        });
      });
    },
  },
};
</script>
