import { debounce } from './modules/debounce.js';

'use strict';

class Home {
  constructor() {
    this.newFeedFormId = "#newFeedForm";
  }

  registerNewFeedHook(){
    $(this.newFeedFormId).submit(function(event) {
      /* stop form from submitting normally */
      event.preventDefault();
      var $form = $(this),
        url = $form.attr('action');

      /* Send the data using post with element id name and name2*/
      var posting = $.post(url, $form.serialize());

      /* Alerts the results */
      posting.done(function(data) {
        alert("success")
      });
      posting.fail(function() {
        alert("failed")
      });
    });
  }
};

$( document ).ready(function() {
  app = new Home;
  app.registerNewFeedHook();
});
