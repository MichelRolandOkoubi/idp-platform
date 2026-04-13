locals {
  environment  = "dev"
  cluster_name = "idp-dev"
  aws_region   = "eu-west-1"
  azs          = ["eu-west-1a", "eu-west-1b"]
}

module "networking" {
  source      = "../../modules/networking"
  environment = local.environment
  azs         = local.azs
}

module "eks" {
  source       = "../../modules/eks"
  cluster_name = local.cluster_name
  environment  = local.environment
  vpc_id       = module.networking.vpc_id
  subnet_ids   = module.networking.private_subnets
}

variable "aws_region"   { default = "eu-west-1" }
variable "environment"  { default = "dev" }