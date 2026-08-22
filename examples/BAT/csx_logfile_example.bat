SET "LOG_FILE=examples\LOGS\csx_logfile_example.txt"
EXECSCRIPT "csx_example.xml" --FilePath "test.txt" --LogFile "%LOG_FILE%"