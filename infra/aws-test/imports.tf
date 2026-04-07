import {
  to = aws_vpc.this
  id = "vpc-0f509fefafc84dba6"
}

import {
  to = aws_internet_gateway.this
  id = "igw-0c27d2c7ce4b62d17"
}

import {
  to = aws_subnet.public["ap-northeast-2a"]
  id = "subnet-06f67ab184687f3e6"
}

import {
  to = aws_subnet.public["ap-northeast-2b"]
  id = "subnet-03af2ec09baf0015d"
}

import {
  to = aws_subnet.private["ap-northeast-2a"]
  id = "subnet-068d4f94971434c43"
}

import {
  to = aws_subnet.private["ap-northeast-2b"]
  id = "subnet-04d6bf5d444b3a6cf"
}

import {
  to = aws_route_table.public
  id = "rtb-013e0f8fa8ad6bccb"
}

import {
  to = aws_route_table.private["ap-northeast-2a"]
  id = "rtb-0d8974104d35f1c74"
}

import {
  to = aws_route_table.private["ap-northeast-2b"]
  id = "rtb-0624c1824990cf48b"
}

import {
  to = aws_route_table.main
  id = "rtb-038039bb5c9b1fdef"
}

import {
  to = aws_route_table_association.public["ap-northeast-2a"]
  id = "subnet-06f67ab184687f3e6/rtb-013e0f8fa8ad6bccb"
}

import {
  to = aws_route_table_association.public["ap-northeast-2b"]
  id = "subnet-03af2ec09baf0015d/rtb-013e0f8fa8ad6bccb"
}

import {
  to = aws_route_table_association.private["ap-northeast-2a"]
  id = "subnet-068d4f94971434c43/rtb-0d8974104d35f1c74"
}

import {
  to = aws_route_table_association.private["ap-northeast-2b"]
  id = "subnet-04d6bf5d444b3a6cf/rtb-0624c1824990cf48b"
}

import {
  to = aws_default_security_group.default
  id = "sg-099203b95b3178ec0"
}
