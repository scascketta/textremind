var ko = require('knockout');
var kov = require('knockout.validation');
var when = require('when');
var sugar  = require('sugar');
var requests  = require('./requests');

var BASE_URL = 'http://127.0.0.1:8000';

ko.validation.configure({
    insertMessages: false
});

// NOTE: https://github.com/knockout/knockout/wiki/Asynchronous-Dependent-Observables
function asyncComputed(evaluator, dependencies) {
    var result = ko.observable();

    ko.computed(function() {
        var ready = true;
        dependencies.forEach(function(e) {
            if (!e() || !e.isValid()) {
                ready = false;
            }
        });
        if (!ready)
            return;
        evaluator().done(result);
    });

    return result;
}

function TextRemind() {
    var self = this;

    self.message = ko.observable('').extend({
        required: true,
        maxLength: { 
            params: 160,
            message: 'Your message is too long, it must be no more than 160 characters.'
        }
    });

    // TODO: normalize phone number to 10-digit plain stuff in different cases
    self.phoneNumber = ko.observable('').extend({
        required: true,
        minLength: 10,
        maxLength: 10
    });

    self.numberVerified = asyncComputed(function() {
        return requests.postJSON(BASE_URL + '/check', { number: self.phoneNumber() })
            .then(function(res) {
                return res.verified;
            });
    }, [self.phoneNumber]);

    self.deliveryTime = ko.observable('').extend({
        required: true,
        validation: {
            validator: function(deliveryTime, _) {
                return Date.future(deliveryTime).isValid();
            },
            message: "Specified time is not valid or is in the past.",
            params: null
        }
    });

    self.password = ko.observable('').extend({
        required: false,
        minLength: 6,
        maxLength: 20
    });
    self.passwordMatches = asyncComputed(function() {
        var data = { number: self.phoneNumber(), password: self.password() };
        return requests.postJSON(BASE_URL + '/check_password', data)
            .then(function(res) {
                return res.matches;
            });
    }, [self.phoneNumber, self.password, self.numberVerified]);
    self.passwordSet = ko.observable(false);

    self.codeSent = ko.observable(false);
    self.code = ko.observable('').extend({
        required: false,
        minLength: 6,
        maxLength: 6
    });
    self.codeMatches = asyncComputed(function() {
        var data = { number: self.phoneNumber(), code: self.code() };
        return requests.postJSON(BASE_URL + '/check_verification', data)
            .then(function(res) {
                return res.valid;
            });
    }, [self.phoneNumber, self.code]);

    self.displayTime = ko.computed(function() {
        return Date.future(self.deliveryTime()).full();
    });

    self.messageSent = ko.observable(false);

    self.errors = ko.validation.group(this);

    self.ready = ko.computed(function() {
        return self.errors().length === 0 && (self.passwordMatches() || self.codeMatches());
    });
}


TextRemind.prototype.schedule = function schedule() {
    var self = this;

    requests.postJSON(BASE_URL + '/schedule', {
        body: self.message(),
        to: self.phoneNumber(),
        time: Date.future(self.deliveryTime()).valueOf() / 1000
    })
    .then(function(res) {
        self.messageSent(true);
    })
    .catch(function(e) {
        self.messageSent(false);
    })
};

TextRemind.prototype.sendCode = function sendCode() {
    var self = this;
    if (self.numberVerified()) return;

    requests.postJSON(BASE_URL + '/send_verification', {
        number: self.phoneNumber()
    })
    .then(function(res) {
        self.codeSent(true);
    })
    .catch(function(e) {
        self.codeSent(false);
        console.error('problem while sending verification code: ', e);
    });
};

TextRemind.prototype.setPassword = function setPassword() {
    var self = this;
    if (!self.password.isValid()) return;

    requests.postJSON(BASE_URL + '/set_password', {
        number: self.phoneNumber(),
        password: self.password()
    }).then(function(res) {
        self.passwordSet(true);
    }).catch(function(e) {
        self.passwordSet(false);
        console.error('problem while setting password: ', e);
    });
};

var tr = window.tr = new TextRemind();

ko.applyBindings(tr, document.getElementById('textremind'));
