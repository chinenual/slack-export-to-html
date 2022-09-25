
# slack-export-to-html

Convert a Slack "export" (zip file containing JSON files) to human readable HTML for archival purposes.

Creates an index.html with links to each public channel's messages and a separate html file for each channel.  Referenced files and attachments are downloaded during the process and used instead of direct references to the original slack URLs.

# To build

```
# go build
```

# To run

* Export data from your Slack workspace via: https://slack.com/help/articles/201658943-Export-your-workspace-data 
* Unzip the resulting zip file
* run the converter:
```
# ./slack-export-to-html -title MyWorkspace Slack export May 24 2020 - Sep 23 2022'  -in /path/to/unzipped/export -out out

```
* produces:
```
# ls -R out
_files		index.html	channelA.html	channelB.html

out/_files:
F013XL8KYJK.jpg		F0186N16R7D.binary	F01EFQ27Z60.png
...
```

# Options
*  **-in** _string_     	- Unzipped location of the Slack export (default ".")
*  **-out** _string_    	- Directory where to write output files (default ".")
 * **-title** _string_    - Title
