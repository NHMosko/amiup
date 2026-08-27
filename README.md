# amiup
### a reliable and simple uptimer
solving the problem of keeping track of your applications' state, with no fancy features

<img width="1000" height="309" alt="amiup usage in terminal" src="https://github.com/user-attachments/assets/8c5820cd-f8d5-45e0-bb8b-a045e5665ffb" />
what the server terminal looks like

## How to use it
### clone the project and run it in a dedicated server
amiup works with a postgres database set in a .env,
you create services for it to keep track of with http post requests,
you keep the server running and everything is done through http,
you may also set these optional env variables (recommended): 
 - an api key 
 - an allowed origin for the requests

when creating a service you give it:
 - a name
 - a URL (the link to the website that is checked)
 - timeout (how long it waits before giving up on the connection)
 - a heartbeat (how often it checks if it's up)
 - strikes (how many times it needs to be down before it is actually considered down)
 - a discord webhook (where it notifies you if the service is down)
all of which can be edited later

the program also keeps track of all requisitions if you wish to check for more information

the endpoints are:
- /services
- /services/{serviceID}
- /services/delete/{serviceID}
- /requisitions

all of the returned jsons in this version are not formatted.

Example POST to /services:
```curl -H "Authorization: ApiKey 1234" localhost:8082/services --data '{"name": "Google", "timeout": 10, "url": "https://google.com", "heartbeat": 5, "strikes": 3, "discord_webhook": ""}'```

### Future
at the moment we only support postgres databases, but my next update will include sqlite support,
other plans for the updates are:
 - making the http endpoints more restful
 - creating a front-end and quality of life features for a more user-friendly experience
 - making it downloadable with 'go install'
