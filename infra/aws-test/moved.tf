moved {
  from = aws_instance.target["ap-northeast-2a"]
  to   = aws_instance.target["devwas"]
}

moved {
  from = aws_instance.target["ap-northeast-2b"]
  to   = aws_instance.target["devapp"]
}
