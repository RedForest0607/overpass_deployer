variable "aws_profile" {
  description = "AWS CLI profile for the ISTOCK-MALL account."
  type        = string
  default     = "ISTOCK-MALL"
}

variable "aws_region" {
  description = "AWS region that contains the ISTOCK-MALL resources."
  type        = string
  default     = "ap-northeast-2"
}

variable "expected_account_id" {
  description = "Guardrail account ID for ISTOCK-MALL."
  type        = string
  default     = "380723148945"
}
