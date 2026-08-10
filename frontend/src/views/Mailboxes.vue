<template>
  <section class="mailboxes">
    <header class="columns page-header">
      <div class="column is-10">
        <h1 class="title is-4">Sending Mailbox Pool</h1>
      </div>
      <div class="column has-text-right">
        <b-button type="is-primary" icon-left="plus" @click="showModal = true">
          Add Mailbox
        </b-button>
      </div>
    </header>

    <b-table :data="mailboxes" :loading="loading" hoverable>
      <b-table-column v-slot="props" field="name" label="Name">
        {{ props.row.name }}
      </b-table-column>
      <b-table-column v-slot="props" field="email" label="Email">
        {{ props.row.email }}
      </b-table-column>
      <b-table-column v-slot="props" field="sent_today" label="Quota Used Today">
        {{ props.row.sent_today }} / {{ props.row.daily_limit }}
      </b-table-column>
      <b-table-column v-slot="props" label="Actions">
        <b-button size="is-small" type="is-danger" icon-left="trash-can-outline" @click="deleteMailbox(props.row.id)" />
      </b-table-column>
    </b-table>

    <b-modal :active.sync="showModal" has-modal-card>
      <div class="modal-card">
        <header class="modal-card-head">
          <p class="modal-card-title">Add Mailbox</p>
        </header>
        <section class="modal-card-body">
          <b-field label="Account Name">
            <b-input v-model="form.name" required placeholder="Sales Rep 1" />
          </b-field>
          <b-field label="Email Address">
            <b-input v-model="form.email" type="email" required placeholder="rep1@outreach.com" />
          </b-field>
          <b-field label="Daily Limit">
            <b-numberinput v-model="form.daily_limit" min="1" max="500" />
          </b-field>
        </section>
        <footer class="modal-card-foot">
          <b-button @click="showModal = false">Cancel</b-button>
          <b-button type="is-primary" @click="saveMailbox">Save</b-button>
        </footer>
      </div>
    </b-modal>
  </section>
</template>

<script>
export default {
  name: 'Mailboxes',
  data() {
    return {
      mailboxes: [],
      loading: false,
      showModal: false,
      form: {
        name: '',
        email: '',
        daily_limit: 50,
      },
    };
  },
  mounted() {
    this.getMailboxes();
  },
  methods: {
    getMailboxes() {
      this.loading = true;
      this.$api.getMailboxes().then((res) => {
        this.mailboxes = res.data;
        this.loading = false;
      });
    },
    saveMailbox() {
      this.$api.createMailbox(this.form).then(() => {
        this.showModal = false;
        this.getMailboxes();
      });
    },
    deleteMailbox(id) {
      this.$utils.confirm('Delete mailbox?', () => {
        this.$api.deleteMailbox(id).then(() => {
          this.getMailboxes();
        });
      });
    },
  },
};
</script>
