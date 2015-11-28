# slack-message-forward
Forward messages between channels

# Dependency
- Redis

# Usage
- Config a slash command integration on Slack
- Copy `credentials.json.default` to `credentials.json` and fill in your Slack config. 
- Run the program
```
go get github.com/hsluo/slack-message-forward
go build
./slack-message-forward
```
  Deployment to OpenShift is supported. Please refer to https://github.com/smarterclayton/openshift-go-cart.

# License
MIT
