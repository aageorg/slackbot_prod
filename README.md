# Choowie, The Slack Bot 

Moves message threads from one channel to another on trigger reaction.

### Installation and usage

Application is prepared for launch in a docker container. 


1. Provide a valid domain name with SSL certificate and save it as "slack_bot_url" value in config.json
1. Create a new application on https://api.slack.com/apps/, configure it using manifest (don't forget to change the domain name to your own)
1. Fill the config.json, using credantials from Slack App homepage (except user and bot tokens)
1. Launch application in docker. Note, app searches the config.json with no path string. Mount it to app work folder.
1. Follow https://{slack_bot_url}/setup to get user token and install the app to your workspace to get bot token. Save both tokens to config.json
1. Restart docker container. Choowie is ready to work.

### The manifest example

```
display_information:
  name: triage-bot
features:
  bot_user:
    display_name: triage-bot
    always_online: false
  slash_commands:
    - command: /showautomoves
      url: https://slackbot.example.com/showautomoves
      description: Show your automoves
      should_escape: true
oauth_config:
  redirect_urls:
    - https://slackbot.example.com/oAuth
  scopes:
    user:
      - reactions:read
    bot:
      - channels:history
      - groups:history
      - chat:write
      - chat:write.customize
      - commands
      - files:write
      - files:read
      - reactions:read
      - users:read
settings:
  event_subscriptions:
    request_url: slackbot.example.com
    user_events:
      - reaction_added
      - reaction_removed
    bot_events:
      - reaction_added
      - reaction_removed
  org_deploy_enabled: false
  socket_mode_enabled: false
  token_rotation_enabled: false
```

### The config file example

```
{
"slack_sign_secret":"1234....",
"slack_client_secret":"1234....",
"slack_client_id":"1234.1234",
"slack_app_id":"ABCD1",
"slack_bot_url":"https://slackbot.example.com",
"slack_bot_token":"xoxb-...",
"slack_user_token":"xoxp-...",
"necessary_votes":0,
"no_remove":true,
"permitted_users":
    [
    "U...",
    "U..."
    ],
"automoves":
    [
    {"from_channel":"C...", "to_channel":"C....", "trigger":"white_check_mark"}
    ]
}
```
