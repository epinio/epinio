'use strict';

class Applications {
  constructor() {
    this.methods = {
      fetchApplications() {
        var that = this
        // TODO: Hardcoded org!
        $.get("/api/v1/orgs/workspace/applications", function(data) {
          that.applications = data;
        }).fail(function() {
          console.log("failed to fetch applications");
        });
      },
      poll() {
        this.fetchApplications();
        setInterval(this.fetchApplications, 5000);
      }
    }
  } 

  data() {
    return { applications: [] }
  }

  mounted() {
    this.poll();
  }

}

const app = Vue.createApp(new Applications)

app.component('application', {
props: ['name', 'status', 'routes', 'services'],
template: `
  <tr>
  <th scope="row">{{ name }}</th>
  <td>{{ status }}</td>
  <td>{{ routes }}</td>
  <td>{{ services }}</td>
  </tr>
`
})

app.mount('#app')
