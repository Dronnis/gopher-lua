local osname = "linux"
if string.find(os.getenv("OS") or "", "Windows") then
  osname = "windows"
end

if osname == "linux" then
  -- travis ci failed to start date command?
  -- assert(os.execute("date") == true)
  local ok = os.execute("date -a")
  assert(ok == true or ok == 1)
else
  local ok = os.execute("date /T")
  assert(ok == true or ok == 0)
  local ok2 = os.execute("md")
  assert(ok2 == false or ok2 == 1)
end

assert(os.getenv("PATH") ~= "")
assert(os.getenv("_____GLUATEST______") == nil)
assert(os.setenv("_____GLUATEST______", "1"))
assert(os.getenv("_____GLUATEST______") == "1")
