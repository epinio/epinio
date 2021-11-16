'use strict';

class NewNamespaceModal {
  constructor() {
    this.newNamespaceFormId = "#newNamespaceForm";
  }

  registerHook(){
    $(this.newNamespaceFormId).submit(function(event) {
      /* stop form from submitting normally */
      event.preventDefault();
      var $form = $(this), url = $form.attr('action');

      var namespaceName = $(event.currentTarget).find("#namespaceName").val();
      var data = { name: namespaceName };
      $.post(url, JSON.stringify(data), function(data){
        window.location.href = "/namespaces/target/"+namespaceName;
      })
      .fail(function(resp) {
        alert("Something went wrong: "+resp.responseText);
        console.log(resp);
      });
    });
  }
};

$( document ).ready(function() {
  var modal = new NewNamespaceModal;
  modal.registerHook();
});


