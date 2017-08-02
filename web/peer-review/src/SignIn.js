import React, { Component } from 'react';
// import logo from './logo.svg';
import { GoogleLogin } from 'react-google-login-component';
import './SignIn.css';

class SignIn extends Component {
  constructor(props, context){
      super(props, context);
      this.onSignIn = this.onSignIn.bind(this);
      this.onFail = this.onFail.bind(this);
  }
  render() {
    return (
      <div>
        <GoogleLogin socialId="852295142503-q581jjeg6d20jc7opv2mgavo9ns6tja3.apps.googleusercontent.com"
                     class="google-login"
                     scope="profile"
                     responseHandler={this.onSignIn}
                     buttonText="Login With Google"/>
      </div>
    );
  }
  onFail(data) {
      console.log("failed", data)
  }
  onSignIn(googleUser) {
    // Useful data for your client-side scripts:
    var profile = googleUser.getBasicProfile();
    console.log("ID: " + profile.getId()); // Don't send this directly to your server!
    console.log('Full Name: ' + profile.getName());
    console.log('Given Name: ' + profile.getGivenName());
    console.log('Family Name: ' + profile.getFamilyName());
    console.log("Image URL: " + profile.getImageUrl());
    console.log("Email: " + profile.getEmail());

    // The ID token you need to pass to your backend:
    var id_token = googleUser.getAuthResponse().id_token;
    console.log("ID Token: " + id_token);

    var xhr = new XMLHttpRequest();
    // TODO - get this port dynamically
    xhr.open('POST', 'http://localhost:3000/tokensignin');
    xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
    xhr.onload = function(resp) {
        if (resp.srcElement.status === 200) {
            console.log('Authorization: Bearer ' + xhr.responseText);
            document.cookie = "auth="+xhr.responseText;
            // window.location.href = "/dash";
        }
    };
    xhr.send('idtoken=' + id_token);
    };
}

export default SignIn;
