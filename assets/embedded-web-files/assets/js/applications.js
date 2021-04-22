'use strict';

class Applications {
  constructor() {
    this.methods = {
      fetchApplications() {
        var that = this
        var org = this.getOrg();
        $.get("/api/v1/orgs/"+org+"/applications", function(data) {
          that.applications = data;
        }).fail(function() {
          console.log("failed to fetch applications");
        });
      },
      poll() {
        this.fetchApplications();
        setInterval(this.fetchApplications, 5000);
      },
      getOrg() {
        var name = "currentOrg";
        var decodedCookie = decodeURIComponent(document.cookie);
        var ca = decodedCookie.split(';');
        for(var i = 0; i <ca.length; i++) {
          var c = ca[i];
          while (c.charAt(0) == ' ') {
            c = c.substring(1);
          }
          if (c.indexOf(name) == 0) {
            return c.substring(name.length+1, c.length);
          }
        }
        return "";
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
props: ['name', 'status', 'route', 'services'],
template: `
  <tr>
  <th scope="row">{{ name }}</th>
  <td>{{ status }}</td>
  <td><a target="_blank" v-bind:href="route">{{ route }}</a></td>
  <td>{{ services }}</td>
  </tr>
`
})

app.mount('#app')
