<template>
  <section class="emails">
    <header class="columns page-header">
      <div class="column is-10">
        <h1 class="title is-4">Sending Email Pool</h1>
      </div>
      <div class="column has-text-right">
        <b-button type="is-primary" icon-left="plus" @click="openCreateModal">
          Add Email Account
        </b-button>
      </div>
    </header>

    <b-table :data="emails" :loading="loading" hoverable>
      <b-table-column v-slot="props" field="name" label="Name">
        {{ props.row.name }}
      </b-table-column>
      <b-table-column v-slot="props" field="email" label="Email">
        {{ props.row.email }}
      </b-table-column>
      <b-table-column v-slot="props" field="sent_today" label="Quota Used Today">
        {{ props.row.sent_today }} / {{ props.row.max_send_per_day || 'Unlimited' }}
      </b-table-column>
      <b-table-column v-slot="props" label="Actions">
        <b-button size="is-small" type="is-info" icon-left="pencil" class="mr-2" @click="openEditModal(props.row)" />
        <b-button size="is-small" type="is-danger" icon-left="trash-can-outline" @click="deleteEmail(props.row.id)" />
      </b-table-column>
    </b-table>

    <b-modal :active.sync="showModal" has-modal-card>
      <div class="modal-card">
        <header class="modal-card-head">
          <p class="modal-card-title">{{ isEditing ? 'Edit Email Account' : 'Add Email Account' }}</p>
        </header>
        <section class="modal-card-body">
          <b-field label="Account Name" label-position="on-border">
            <b-input v-model="form.name" required placeholder="Sales Rep 1" />
          </b-field>
          <b-field label="Email Address" label-position="on-border">
            <b-input v-model="form.email" type="email" required placeholder="rep1@outreach.com" />
          </b-field>
          <div class="columns">
            <div class="column is-12">
              <b-field label="Daily Sending Quota" label-position="on-border" message="0 = unlimited">
                <b-numberinput v-model="form.max_send_per_day" min="0" max="100000" />
              </b-field>
            </div>
          </div>
          <b-field label="Signature" label-position="on-border" message="Default signature for messages sent from this email account">
            <b-input v-model="form.signature" type="textarea" placeholder="Best regards,\nSales Rep Name" />
          </b-field>
        </section>
        <footer class="modal-card-foot">
          <b-button @click="showModal = false">Cancel</b-button>
          <b-button type="is-primary" @click="saveEmail">Save</b-button>
        </footer>
      </div>
    </b-modal>
  </section>
</template>

<script>
export default {
  name: 'Emails',
  data() {
    return {
      emails: [],
      loading: false,
      showModal: false,
      isEditing: false,
      form: {
        id: null,
        name: '',
        email: '',
        max_send_per_day: 0,
        signature: '',
      },
    };
  },
  mounted() {
    this.getEmails();
  },
  methods: {
    getEmails() {
      this.loading = true;
      this.$api.getEmails().then((res) => {
        this.emails = res.data;
        this.loading = false;
      });
    },
    openCreateModal() {
      this.isEditing = false;
      this.form = {
        id: null,
        name: '',
        email: '',
        max_send_per_day: 0,
        signature: '',
      };
      this.showModal = true;
    },
    openEditModal(row) {
      this.isEditing = true;
      this.form = {
        id: row.id,
        name: row.name,
        email: row.email,
        max_send_per_day: row.max_send_per_day || 0,
        signature: row.signature || '',
      };
      this.showModal = true;
    },
    saveEmail() {
      if (this.isEditing) {
        this.$api.updateEmail(this.form.id, this.form).then(() => {
          this.showModal = false;
          this.getEmails();
        });
      } else {
        this.$api.createEmail(this.form).then(() => {
          this.showModal = false;
          this.getEmails();
        });
      }
    },
    deleteEmail(id) {
      this.$utils.confirm('Delete email account?', () => {
        this.$api.deleteEmail(id).then(() => {
          this.getEmails();
        });
      });
    },
  },
};
</script>
