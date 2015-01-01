var requests = {
    requestID: 0,
    send: function(method, url, data, headers) {
        var self = this,
            deferred = when.defer(),
            xhr = new XMLHttpRequest(),
            done = false,
            lowerMethod = method.toLowerCase(),
            isGetRequest = lowerMethod === 'get',
            isDeleteRequest = lowerMethod === 'delete',
            requestID;

        this.requestID += 1;
        requestID = this.requestID;

        if (isGetRequest && data && Object.keys(data).length > 0) {
            url += '?' + this.serializeParams(data);
        }

        xhr.onload = function(event) {
            // Sometimes webkit gets a little trigger happy and calls this multiple times
            // https://code.google.com/p/chromium/issues/detail?id=159827
            if (done) return;
            done = true;

            var logFn = xhr.status > 299 ? console.error : console.log;
            // logFn.call(undefined, requestID.toString(), '[' + method.toUpperCase() + ']', url, '=>', xhr.status);
            console.log(requestID.toString(), '[' + method.toUpperCase() + ']', url, '=>', xhr.status);

            // Sometimes different environments don't use case-insensitive matching
            var contentType = xhr.getResponseHeader('content-type') || xhr.getResponseHeader('Content-type');

            if (xhr.status >= 200 && xhr.status < 400) {
                var result = null;
                if (/application\/json/i.test(contentType)) {
                    result = JSON.parse(xhr.responseText);
                } else if (/text\/xml/i.test(contentType)) {
                    var parser = new DOMParser();
                    result = parser.parseFromString(xhr.responseText, "text/xml");
                } else {
                    result = xhr.responseText;
                }
                deferred.resolve(result);
            }
            else {
                deferred.reject(xhr.responseText);
            }
        };

        xhr.onerror = function() {
            deferred.reject(new Error('Unable to connect to the server.'));
        };

        xhr.ontimeout = function() {
            deferred.reject(new Error('It took too long for the server to respond.'));
        };

        xhr.open(method, url, true);

        if (headers) {
            for (var attr in headers) {
               if (headers.hasOwnProperty(attr)) {
                   xhr.setRequestHeader(attr, headers[attr]);
               }
            }
        }

        var requestData = data !== undefined && !isGetRequest ? JSON.stringify(data) : undefined;

        // NOTE: nginx gets pissy if the content-length header is set to 0 so we send along garbage data
        if (isDeleteRequest && (!requestData || requestData.length === 0)) {
            requestData = '1';
        }

        xhr.timeout = 1000 * 30;
        xhr.send(requestData);

        return deferred.promise;
    },
    serializeParams: function(params) {
        var keyValues = [];

        function push(key, val) {
            keyValues.push(encodeURIComponent(key) + '=' + encodeURIComponent(val));
        }

        for (var key in params) {
            if (params.hasOwnProperty(key)) {
                var val = params[key];

                if (Array.isArray(val)) {
                    var pushArrayVal = push.bind(undefined, key);
                    val.forEach(pushArrayVal);
                }
                else {
                    push(key, val);
                }
            }
        }

        return keyValues.join('&');
    }
};

requests.get = requests.send.bind(requests, 'GET');
requests.put = requests.send.bind(requests, 'PUT');
requests.patch = requests.send.bind(requests, 'PATCH');
requests.post = requests.send.bind(requests, 'POST');
requests.delete = requests.send.bind(requests, 'DELETE');

var ko = require('knockout');
var kov = require('knockout.validation');
var when = require('when');
var sugar  = require('sugar');
var sugar  = require('sugar');
require('es6-promise').polyfill();
require('fetch');

var BASE_URL = 'http://127.0.0.1:8000';

ko.validation.configure({
    insertMessages: false
});

// NOTE: https://github.com/knockout/knockout/wiki/Asynchronous-Dependent-Observables
function asyncComputed(evaluator, owner, dependencies) {
    var result = ko.observable();

    ko.computed(function() {
        var ready = true;
        dependencies.forEach(function(e) {
            if (!e()) {
                ready = false;
            }
        });
        if (!ready)
            return;
        var promise = evaluator.call(owner);
        promise.done(result);
    });

    return result;
}

function TextRemind() {
    var self = this;

    self.inputMessage = ko.observable('').extend({
        required: true,
        maxLength: { 
            params: 160,
            message: 'Your message is too long, it must be no more than 160 characters.'
        }
    });

    // TODO: normalize phone number to 10-digit plain stuff in different cases
    self.inputNumber = ko.observable('').extend({
        required: true,
        minLength: 10,
        maxLength: 10
    });

    self.numberVerified = asyncComputed(function() {
        return requests.post(BASE_URL + '/check', { number: self.inputNumber() }, {'Accept': 'application/json'});
    }, self, [self.inputNumber]);

    // ko.computed(function() {
    //     if (self.inputNumber().length == 0) return;

    //     fetch(BASE_URL + '/check', {
    //         method: 'post',
    //         headers: { 'Accept': 'application/json' },
    //         body: JSON.stringify({ number: self.inputNumber() })
    //     }).then(function(res) { return res.json() })
    //     .then(function(json) {
    //         self.numberVerified(json['verified']);
    //     });
    // });

    self.inputTime = ko.observable('').extend({
        required: true,
        validation: {
            validator: function(inputTime, _) {
                return Date.future(inputTime).isValid();
            },
            message: "Specified time is not valid or is in the past.",
            params: null
        }
    });

    self.inputPassword = ko.observable('').extend({
        required: true,
        minLength: 6,
        maxLength: 20
    });
    self.passwordMatches = ko.observable(false);
    ko.computed(function() {
        if (self.inputPassword().length == 0)return;

        fetch(BASE_URL + '/check_password', {
            method: 'post',
            headers: { 'Accept': 'application/json' },  
            body: JSON.stringify({ number: self.inputNumber(), password: self.inputPassword() })
        }).then(function(res) { return res.json() })
        .then(function(json) {
            self.passwordMatches(json['matches']);
        });
    });

    self.codeSent = ko.observable(false);
    self.inputCode = ko.observable('').extend({
        required: true,
        minLength: 6,
        maxLength: 6
    });
    self.codeMatches = ko.observable(false);
    ko.computed(function() {
        if (self.inputCode().length == 0 || self.inputNumber().length == 0) return;

        fetch(BASE_URL + '/check_verification', {
            method: 'post',
            headers: { 'Accept': 'application/json' },  
            body: JSON.stringify({ number: self.inputNumber(), code: self.inputCode() })
        }).then(function(res) { return res.json(); })
        .then(function(json) {
            self.codeMatches(json['valid']);
        });
    });

    self.passwordSet = ko.observable(false);
    self.inputSetPassword = ko.observable('').extend({
        required: true,
        minLength: 6,
        maxLength: 20
    });

    self.displayTime = ko.computed(function() {
        return Date.future(self.inputTime()).full();
    }, this);

    self.errors = ko.validation.group(this);

    self.ready = ko.computed(function() {
        return self.errors().length == 0;
    }, this);
};


TextRemind.prototype.schedule = function() {
    var self = this;

    var data = {
        body: self.inputMessage(),
        to: self.inputNumber(),
        time: Date.future(self.inputTime()).valueOf() / 1000
    };

    fetch(BASE_URL + '/schedule', {
        method: 'post',
        headers: { 'Accept': 'application/json' },
        body: JSON.stringify(data)
    }).then(function(res) {
        return res.json();
    }).then(function(json) {
        console.log('res.json().data: ', json);
    }).catch(function(ex) {
        console.log('error!: ', ex);
    })
};

TextRemind.prototype.sendCode = function() {
    var self = this;
    if (self.numberVerified()) return;

    fetch(BASE_URL + '/send_verification', {
        method: 'post',
        headers: { 'Accept': 'application/json' },
        body: JSON.stringify({ number: self.inputNumber() })
    }).then(function(res) {
        self.codeSent(true);
    }).catch(function(e) {
        console.log('problem while sending verification code: ', e);
        self.codeSent(false);
    });
}

TextRemind.prototype.setPassword = function() {
    var self = this;
    if (self.inputSetPassword().length == 0) return;

    fetch(BASE_URL + '/set_password', {
        method: 'post',
        headers: { 'Accept': 'application/json' },
        body: JSON.stringify({ number: self.inputNumber(), password: self.inputSetPassword() })
    }).then(function(res) {
        self.passwordSet(true);
    }).catch(function(e) {
        console.log('problem while setting password: ', e);
        self.passwordSet(false);
    });
}

var tr = window.tr = new TextRemind();

ko.applyBindings(tr, document.getElementById('textremind'));
