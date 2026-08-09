<template>
  <form @submit.prevent="onSubmit">
    <div class="modal-card content" style="width: auto">
      <header class="modal-card-head">
        <h4 v-if="isEditing">
          {{ data.name || data.email }}
        </h4>
        <h4 v-else>
          New Contact
        </h4>
      </header>

      <section expanded class="modal-card-body">
        <div class="columns">
          <div class="column is-6">
            <b-field label="Name" label-position="on-border">
              <b-input :maxlength="200" v-model="form.name" name="name" placeholder="Full Name" />
            </b-field>
          </div>
          <div class="column is-6">
            <b-field label="Phone Number" label-position="on-border">
              <b-input :maxlength="50" v-model="form.phone" name="phone" placeholder="+15550192834" />
            </b-field>
          </div>
        </div>

        <b-field label="Email Address" label-position="on-border">
          <b-input :maxlength="200" v-model="form.email" name="email" placeholder="email@domain.com" required />
        </b-field>

        <b-field label="Attributes (JSON)" label-position="on-border">
          <b-input type="textarea" v-model="form.attribsJSON" rows="4" placeholder="{ 'company': 'Acme Inc' }" />
        </b-field>
      </section>

      <footer class="modal-card-foot align-right">
        <b-button @click="$parent.close()">
          Cancel
        </b-button>
        <b-button type="is-primary" native-type="submit" :loading="loading.subscribers">
          Save Contact
        </b-button>
      </footer>
    </div>
  </form>
</template>

<script>
import { mapState } from 'vuex';

export default {
  name: 'ContactForm',
  props: {
    data: {
      type: Object,
      default: () => ({}),
    },
    isEditing: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    return {
      form: {
        id: null,
        name: '',
        email: '',
        phone: '',
        attribsJSON: '{}',
        status: 'enabled',
      },
    };
  },
  computed: {
    ...mapState(['loading']),
  },
  watch: {
    data: {
      immediate: true,
      handler(val) {
        if (val && val.id) {
          this.form = {
            id: val.id,
            name: val.name || '',
            email: val.email || '',
            phone: val.phone || '',
            attribsJSON: JSON.stringify(val.attribs || {}, null, 2),
            status: val.status || 'enabled',
          };
        } else {
          this.form = {
            id: null,
            name: '',
            email: '',
            phone: '',
            attribsJSON: '{}',
            status: 'enabled',
          };
        }
      },
    },
  },
  methods: {
    onSubmit() {
      let attribs = {};
      try {
        attribs = JSON.parse(this.form.attribsJSON || '{}');
      } catch (e) {
        this.$utils.toast('Invalid JSON attributes');
        return;
      }

      const payload = {
        id: this.form.id,
        name: this.form.name,
        email: this.form.email,
        phone: this.form.phone,
        attribs,
        status: this.form.status,
      };

      const action = this.isEditing ? this.$api.updateContact : this.$api.createContact;
      action(payload).then(() => {
        this.$utils.toast(this.isEditing ? 'Contact updated' : 'Contact created');
        this.$emit('finished');
        this.$parent.close();
      });
    },
  },
};
</script>
