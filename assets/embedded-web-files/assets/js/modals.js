'use strict';

class NewOrgModal {
  constructor() {
    this.newOrgFormId = "#newOrgForm";
  }

  registerHook(){
    $(this.newOrgFormId).submit(function(event) {
      /* stop form from submitting normally */
      event.preventDefault();
      var $form = $(this), url = $form.attr('action');

      orgName = $(event.currentTarget).find("#orgName").val();
      var data = { name: orgName };
      $.post(url, JSON.stringify(data), function(data){
        window.location.href = "/orgs/target/"+orgName;
      })
      .fail(function(resp) {
        alert("Something went wrong: "+resp.responseText);
        console.log(resp);
      });
    });
  }
};

$( document ).ready(function() {
  var modal = new NewOrgModal;
  modal.registerHook();
});


