# scele_gcal

Fetches SCeLe assignments and adds them to your Google Tasks.

## Limitations

- You need to setup your own OAuth authentication using Google Cloud. Not planning to host this at the moment.
- Only adds tasks to the first task list on your Google Tasks.
- Only gets current month's tasks from SCeLe.
- Codebase is very messy and slow, will try to refactor later.

## Assumptions 

(re: Not gonna bother explaining how to do these things)
- You have set up the OAuth2 credentials and enabled the Google Tasks API on Google Cloud.
- You have downloaded the OAuth client json and saved it as `config.json`

## Warnings

- DO NOT SHARE YOUR TOKENS AND SECRETS!
- Since this will interact with your Google account, please make sure you set up your OAuth correctly and are comfortable with whatever the code here is doing.
- Do not request to SCeLe too frequently (I think ~15-30 minutes is fine).

## Example Config JSON

scele_config.json

```json
{
    "username": "",
    "password": "",
    "excluded_courses": [
        // enter course id to ignore
    ],
    "excluded_keywords": [
        // enter keywords to ignore (e.g. "Attendance", "Feedback")
    ]
}
```

## Setup

```
git clone --depth 1 git@github.com:kawijayaa/scele_gcal
go get
go run .
```

## How To Use

0. Again, I will assume you have done all the steps above.
1. If you are running this for the first time, it will prompt you to open a link to authenticate yourself using Google. Open the link on your browser
2. Login using the Google account you want your tasks to be in.
3. Copy the `code` parameter after being redirected. The link usually looks like this `http://localhost/?state=state-token&code=<YOUR-CODE-HERE-COPY-THIS-PART>&scope=....`
4. ???
5. Profit!

## References on Setup

- https://developers.google.com/workspace/guides/configure-oauth-consent
- https://developers.google.com/workspace/guides/create-credentials
